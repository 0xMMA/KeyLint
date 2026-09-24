package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"keylint/internal/logger"
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
	if req.Temperature != nil {
		params.Temperature = anthropic.Float(*req.Temperature)
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
		return Response{}, mapAnthropicError(attempts, req.Model, err)
	}

	// A reply can arrive as several text blocks; taking only the first would
	// silently truncate it. Thinking blocks are counted, not read: with the
	// default display they arrive empty, and they are never the answer.
	var builder strings.Builder
	thinking := false
	for _, block := range message.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			builder.WriteString(b.Text)
		case anthropic.ThinkingBlock, anthropic.RedactedThinkingBlock:
			thinking = true
		}
	}
	text := builder.String()

	// A truncated answer would be pasted over the user's selection as a
	// half-written sentence — the same call the Claude Code client makes.
	switch message.StopReason {
	case anthropic.StopReasonMaxTokens, anthropic.StopReasonModelContextWindowExceeded:
		// Usage carries no user text, so it may be logged at Warn: it is what
		// tells a reader whether the budget went on thinking or on the answer.
		logger.Warn("llm: output limit reached", "feature", c.cfg.Feature, "provider", anthropicProvider.id,
			"model", req.Model, "stop_reason", string(message.StopReason), "max_tokens", req.MaxTokens,
			"output_tokens", message.Usage.OutputTokens, "thinking", thinking, "answer_bytes", len(text))
		if message.StopReason == anthropic.StopReasonMaxTokens && thinking && strings.TrimSpace(text) == "" {
			return Response{}, fmt.Errorf("%s: %s", anthropicProvider.name, outputLimitThinkingMessage)
		}
		return Response{}, fmt.Errorf("%s: %s", anthropicProvider.name, outputLimitMessage)
	case anthropic.StopReasonRefusal:
		return Response{}, fmt.Errorf("%s declined the request", anthropicProvider.name)
	}
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
func mapAnthropicError(attempts *httpAttempts, model string, err error) error {
	// The caller giving up is not a provider failure — see mapOpenAIError.
	if isContextError(err) {
		return transportError(anthropicProvider, err)
	}
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		return apiErrorForModel(anthropicProvider, apiErr.StatusCode, model, apiErr.RawJSON())
	}
	// The SDK could not decode the failure, but the middleware saw the status.
	if status := attempts.statusOrZero(); status >= 400 {
		return apiErrorForModel(anthropicProvider, status, model, "")
	}
	return transportError(anthropicProvider, err)
}
