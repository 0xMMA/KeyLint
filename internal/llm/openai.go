package llm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// defaultOpenAIBaseURL is the public OpenAI API host. It is always passed
// explicitly: openai-go has no WithoutEnvironmentDefaults, so leaving the base
// URL unset lets OPENAI_BASE_URL redirect the request — with the user's key —
// to a host no KeyLint setting shows.
const defaultOpenAIBaseURL = "https://api.openai.com"

// openAIProvider carries the ID used in logs and the name used in errors.
var openAIProvider = provider{id: ProviderOpenAI, name: "OpenAI"}

type openAIClient struct {
	cfg Config
}

func newOpenAI(cfg Config) Client { return &openAIClient{cfg: cfg} }

func (c *openAIClient) Complete(ctx context.Context, req Request) (Response, error) {
	return completeViaOpenAI(ctx, c.cfg, openAIProvider, resolveBaseURL(c.cfg.BaseURL, defaultOpenAIBaseURL), req)
}

// completeViaOpenAI serves both OpenAI itself and every OpenAI-compatible
// endpoint — today that is Ollama, which speaks this dialect under /v1. The
// base URL is always explicit; see defaultOpenAIBaseURL.
func completeViaOpenAI(ctx context.Context, cfg Config, p provider, baseURL string, req Request) (Response, error) {
	if req.Model == "" {
		return Response{}, fmt.Errorf("%s: model is required", p.name)
	}

	params := openai.ChatCompletionNewParams{
		// shared.ChatModel is a string alias: the caller's ID goes through as is.
		Model: shared.ChatModel(req.Model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(req.System),
			openai.UserMessage(req.User),
		},
	}
	if len(req.JSONSchema) > 0 {
		schema, err := schemaObject(req.JSONSchema)
		if err != nil {
			return Response{}, fmt.Errorf("%s: %w", p.name, err)
		}
		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name: schemaName,
					// Strict makes the model follow the schema exactly rather
					// than approximately; the caller's defensive parse stays as
					// a fallback for endpoints that only approximate it.
					Strict: openai.Bool(true),
					Schema: schema,
				},
			},
		}
	}
	logRequest(cfg, p, req.Model, params)

	attempts := &httpAttempts{cfg: cfg, provider: p}
	// The service method has a pointer receiver, so the client needs a variable.
	client := openAISDKClient(cfg, baseURL, attempts)
	completion, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return Response{}, mapOpenAIError(p, attempts, err)
	}
	if len(completion.Choices) == 0 {
		return Response{}, fmt.Errorf("%s returned no choices", p.name)
	}

	choice := completion.Choices[0]
	// With a strict schema a refusal is a first-class outcome: the content is
	// empty and the reason is in its own field.
	if refusal := strings.TrimSpace(choice.Message.Refusal); refusal != "" {
		return Response{}, fmt.Errorf("%s declined the request: %s", p.name, refusal)
	}
	// A truncated answer would be pasted over the user's selection as a
	// half-written sentence — the same call the Claude Code client makes.
	if choice.FinishReason == "length" {
		return Response{}, fmt.Errorf("%s ran out of output tokens before finishing", p.name)
	}

	text := choice.Message.Content
	if strings.TrimSpace(text) == "" {
		return Response{}, fmt.Errorf("%s returned an empty result", p.name)
	}
	logResponse(cfg, p, text)
	return Response{Text: text}, nil
}

// openAISDKClient builds the SDK client for an OpenAI-compatible endpoint.
func openAISDKClient(cfg Config, baseURL string, attempts *httpAttempts) openai.Client {
	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
		option.WithHTTPClient(cfg.httpClient()),
		option.WithMaxRetries(sdkMaxRetries),
		option.WithMiddleware(attempts.middleware),
		// Always explicit, so OPENAI_BASE_URL cannot redirect the request. The
		// SDK appends the endpoint path, so the host needs a trailing slash.
		option.WithBaseURL(baseURL + "/v1/"),
		// openai-go also reads OPENAI_ORG_ID and OPENAI_PROJECT_ID from the
		// environment and would send them to whatever host is configured —
		// including a local Ollama.
		option.WithHeaderDel("OpenAI-Organization"),
		option.WithHeaderDel("OpenAI-Project"),
	}
	// OPENAI_CUSTOM_HEADERS is arbitrary: whatever it names is added to every
	// request, to any host. The SDK offers no opt-out, so the same variable is
	// parsed here and each header it sets is deleted again.
	for _, header := range environmentCustomHeaders() {
		opts = append(opts, option.WithHeaderDel(header))
	}
	if timeout := cfg.requestTimeout(); timeout > 0 {
		opts = append(opts, option.WithRequestTimeout(timeout))
	}
	for _, header := range fingerprintHeaders {
		opts = append(opts, option.WithHeaderDel(header))
	}
	opts = append(opts, option.WithHeader("User-Agent", userAgent))
	return openai.NewClient(opts...)
}

// environmentCustomHeaders returns the header names OPENAI_CUSTOM_HEADERS sets,
// parsed the way openai-go parses them: newline-separated "Name: value" lines.
func environmentCustomHeaders() []string {
	raw, ok := os.LookupEnv("OPENAI_CUSTOM_HEADERS")
	if !ok {
		return nil
	}
	var names []string
	for _, line := range strings.Split(raw, "\n") {
		name, _, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// mapOpenAIError turns an SDK error into the wording the user sees. The raw body
// stays out of it — see statusMessage.
func mapOpenAIError(p provider, attempts *httpAttempts, err error) error {
	// The caller giving up is not a provider failure, and callers test for it
	// with errors.Is — so it must not become a status even if an earlier
	// attempt saw one.
	if isContextError(err) {
		return transportError(p, err)
	}
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		return apiError(p, apiErr.StatusCode, apiErr.RawJSON())
	}
	// OpenAI-compatible servers that answer {"error":"<string>"} rather than an
	// object defeat the SDK's typed error, and the status would otherwise be
	// lost behind a JSON-unmarshal message. The middleware saw it.
	if status := attempts.statusOrZero(); status >= 400 {
		return apiError(p, status, "")
	}
	return transportError(p, err)
}
