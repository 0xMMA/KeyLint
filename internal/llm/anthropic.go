package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

	// An alias is resolved here, and the effort and thinking options are
	// checked against what the model accepts. Both need the account's model
	// listing, which is cached (ten minutes; thirty seconds after a failure).
	// A request naming a pinned ID with neither option set never asks for it.
	// Fix on a pinned Haiku does ask — it sets NoThinking — and pays at most
	// one bounded listing call per cache lifetime for it.
	model := NormalizeFamily(req.Model)
	effort := req.Effort
	if effort != "" && !IsEffortLevel(effort) {
		// A hand-edited settings file is the only way here. Sending it would
		// be a guaranteed 400 on every completion.
		logger.Warn("llm: unknown effort level ignored", "feature", c.cfg.Feature, "effort", effort)
		effort = ""
	}
	noThinking := req.NoThinking
	if IsModelFamily(model) || effort != "" || noThinking {
		catalogue, live := anthropicCatalogue(ctx, c.cfg)
		if IsModelFamily(model) {
			model = resolveAnthropicFamily(c.cfg, model, catalogue, live)
		}
		if entry, ok := findModel(catalogue, model); ok {
			if effort != "" && !entry.acceptsEffort(effort) {
				// Info, not Warn: the user chose a level and a model that does
				// not take it, and the request is still the right one to send.
				logger.Info("llm: effort not supported by this model, sent without it",
					"feature", c.cfg.Feature, "model", model, "effort", effort)
				effort = ""
			}
			if noThinking && !entry.acceptsThinkingOff() {
				noThinking = false
			}
		}
	}

	params := anthropic.MessageNewParams{
		// anthropic.Model is a string alias: the caller's ID goes through as is,
		// so a model released after this SDK version still works.
		Model:     anthropic.Model(model),
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
		params.OutputConfig.Format = anthropic.JSONOutputFormatParam{Schema: schema}
	}
	if effort != "" {
		params.OutputConfig.Effort = anthropic.OutputConfigEffort(effort)
	}
	if noThinking {
		disabled := anthropic.NewThinkingConfigDisabledParam()
		params.Thinking = anthropic.ThinkingConfigParamUnion{OfDisabled: &disabled}
	}

	logRequest(c.cfg, anthropicProvider, model, params)

	attempts := newHTTPAttempts(ctx, c.cfg, anthropicProvider)
	// The service method has a pointer receiver, so the client needs a variable.
	client := c.client(attempts)
	message, err := client.Messages.New(ctx, params)
	if err != nil && (effort != "" || noThinking) && rejectedOption(err) {
		// The listing did not describe this model, or described it wrongly,
		// and the model refused the effort or thinking option. Once more
		// without them: the user's text still gets fixed, on the model's own
		// default, rather than failing on a setting they may not remember
		// making. Warn, because the setting is now silently doing nothing.
		logger.Warn("llm: model rejected the effort or thinking option, retried without it",
			"feature", c.cfg.Feature, "model", model, "effort", effort, "no_thinking", noThinking)
		params.OutputConfig.Effort = ""
		params.Thinking = anthropic.ThinkingConfigParamUnion{}
		attempts = newHTTPAttempts(ctx, c.cfg, anthropicProvider)
		client = c.client(attempts)
		message, err = client.Messages.New(ctx, params)
	}
	if err != nil {
		return Response{}, mapAnthropicError(attempts, model, err)
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
			"model", model, "stop_reason", string(message.StopReason), "max_tokens", req.MaxTokens,
			"output_tokens", message.Usage.OutputTokens, "thinking", thinking, "answer_bytes", len(text))
		if message.StopReason == anthropic.StopReasonMaxTokens && thinking {
			return Response{}, fmt.Errorf("%s: %w", anthropicProvider.name, ErrOutputLimitThinking)
		}
		return Response{}, fmt.Errorf("%s: %w", anthropicProvider.name, ErrOutputLimit)
	case anthropic.StopReasonRefusal:
		return Response{}, fmt.Errorf("%s declined the request", anthropicProvider.name)
	}
	if strings.TrimSpace(text) == "" {
		return Response{}, fmt.Errorf("%s returned no text content", anthropicProvider.name)
	}
	logResponse(c.cfg, anthropicProvider, text)
	answered := string(message.Model)
	if answered == "" {
		answered = model
	}
	return Response{Text: text, Model: answered}, nil
}

// findModel looks a model ID up in the listing.
func findModel(models []anthropicModel, id string) (anthropicModel, bool) {
	for _, m := range models {
		if m.ID == id {
			return m, true
		}
	}
	return anthropicModel{}, false
}

// rejectedOption reports whether a failure is the model refusing the effort or
// thinking option — "This model does not support the effort parameter.",
// "does not support effort level 'xhigh'" (verified against the API on
// 2026-10-08). The raw body is matched, never shown: see statusMessage.
func rejectedOption(err error) bool {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		return false
	}
	body := strings.ToLower(apiErr.RawJSON())
	return strings.Contains(body, "effort") || strings.Contains(body, "thinking")
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
