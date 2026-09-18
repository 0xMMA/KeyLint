package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// anthropicProvider carries the ID used in logs and the name used in errors.
// The name is "Claude" because that is what the UI calls this provider.
var anthropicProvider = provider{id: ProviderClaude, name: "Claude"}

type anthropicClient struct {
	cfg Config
}

func newAnthropic(cfg Config) Client { return &anthropicClient{cfg: cfg} }

func (c *anthropicClient) Complete(ctx context.Context, req Request) (Response, error) {
	if req.Model == "" {
		return Response{}, fmt.Errorf("%s: model is required", anthropicProvider.name)
	}
	if req.MaxTokens <= 0 {
		return Response{}, fmt.Errorf("%s: max_tokens is required", anthropicProvider.name)
	}

	params := anthropic.MessageNewParams{
		// anthropic.Model is a string alias: the caller's ID goes through as is,
		// so a model released after this SDK version still works.
		Model:     anthropic.Model(req.Model),
		MaxTokens: int64(req.MaxTokens),
		System:    []anthropic.TextBlockParam{{Text: req.System}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.User)),
		},
	}
	if len(req.JSONSchema) > 0 {
		schema, err := schemaObject(req.JSONSchema)
		if err != nil {
			return Response{}, fmt.Errorf("%s: %w", anthropicProvider.name, err)
		}
		params.OutputConfig = anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		}
	}

	logRequest(c.cfg, anthropicProvider, req.Model, params)

	attempts := &httpAttempts{cfg: c.cfg, provider: anthropicProvider}
	// The service method has a pointer receiver, so the client needs a variable.
	client := c.client(attempts)
	message, err := client.Messages.New(ctx, params)
	if err != nil {
		return Response{}, mapAnthropicError(attempts, err)
	}

	// A truncated answer would be pasted over the user's selection as a
	// half-written sentence — the same call the Claude Code client makes.
	switch message.StopReason {
	case anthropic.StopReasonMaxTokens, anthropic.StopReasonModelContextWindowExceeded:
		return Response{}, fmt.Errorf("%s: %s", anthropicProvider.name, outputLimitMessage)
	case anthropic.StopReasonRefusal:
		return Response{}, fmt.Errorf("%s declined the request", anthropicProvider.name)
	}

	// A reply can arrive as several text blocks; taking only the first would
	// silently truncate it.
	var builder strings.Builder
	for _, block := range message.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			builder.WriteString(text.Text)
		}
	}
	text := builder.String()
	if strings.TrimSpace(text) == "" {
		return Response{}, fmt.Errorf("%s returned no text content", anthropicProvider.name)
	}
	logResponse(c.cfg, anthropicProvider, text)
	return Response{Text: text}, nil
}

// client builds the SDK client per call. Construction is cheap, and it keeps the
// HTTP client and base URL the caller configured authoritative.
func (c *anthropicClient) client(attempts *httpAttempts) anthropic.Client {
	opts := []option.RequestOption{
		// Without this the SDK silently picks up ANTHROPIC_BASE_URL,
		// ANTHROPIC_AUTH_TOKEN, ANTHROPIC_CUSTOM_HEADERS and a profile under
		// ~/.anthropic — so on a machine set up for a gateway, the user's
		// clipboard text and the key KeyLint holds would go to a host no
		// setting shows. KeyLint decides where its requests go; this is the
		// same stance claudecode.go takes when it strips ANTHROPIC_* from the
		// CLI's environment.
		option.WithoutEnvironmentDefaults(),
		option.WithAPIKey(c.cfg.APIKey),
		option.WithHTTPClient(c.cfg.httpClient()),
		option.WithMaxRetries(sdkMaxRetries),
		option.WithMiddleware(attempts.middleware),
	}
	if timeout := c.cfg.requestTimeout(); timeout > 0 {
		opts = append(opts, option.WithRequestTimeout(timeout))
	}
	for _, header := range fingerprintHeaders {
		opts = append(opts, option.WithHeaderDel(header))
	}
	opts = append(opts, option.WithHeader("User-Agent", userAgent))
	// Empty leaves the SDK on api.anthropic.com; a value points at a proxy or,
	// in tests, at an httptest server.
	if base := resolveBaseURL(c.cfg.BaseURL, ""); base != "" {
		// The SDK appends the endpoint path, so the host needs a trailing slash.
		opts = append(opts, option.WithBaseURL(base+"/"))
	}
	return anthropic.NewClient(opts...)
}

// mapAnthropicError turns an SDK error into the wording the user sees. The raw
// body stays out of it — see statusMessage.
func mapAnthropicError(attempts *httpAttempts, err error) error {
	// The caller giving up is not a provider failure — see mapOpenAIError.
	if isContextError(err) {
		return transportError(anthropicProvider, err)
	}
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		return apiError(anthropicProvider, apiErr.StatusCode, apiErr.RawJSON())
	}
	// The SDK could not decode the failure, but the middleware saw the status.
	if status := attempts.statusOrZero(); status >= 400 {
		return apiError(anthropicProvider, status, "")
	}
	return transportError(anthropicProvider, err)
}
