// Package llm is the single place in KeyLint that knows how an LLM provider's
// HTTP API is shaped. Callers build a Request, pick a provider ID and get a
// Response back; features such as enhance and pyramidize must not contain
// provider-specific request or response handling.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"keylint/internal/logger"
)

// Provider IDs. These match the values persisted in settings (ActiveProvider).
const (
	ProviderOpenAI = "openai"
	ProviderClaude = "claude"
	ProviderOllama = "ollama"
)

// Request is a single-turn completion request.
type Request struct {
	// System is the system prompt. Providers without a dedicated system field
	// (Ollama) prepend it to User, joined by Config.PromptSeparator.
	System string
	// User is the user message.
	User string
	// Model is the provider-specific model ID and is required — defaults live
	// at the call site until they move into settings (#33 step 4).
	Model string
	// MaxTokens caps the response length. Only providers whose API requires it
	// (Anthropic) send it; the others keep the request shape they had before.
	MaxTokens int
	// JSONMode asks the provider to constrain output to a JSON object. Only
	// OpenAI enforces it; for the others the caller parses defensively.
	JSONMode bool
}

// Response is the text of a completion.
type Response struct {
	Text string
}

// Client talks to exactly one provider.
type Client interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

// Config carries what a client needs that is not part of a single Request.
// API key resolution stays with the caller — settings owns the keyring.
type Config struct {
	// APIKey authenticates the request. Unused by providers that need no key.
	APIKey string
	// BaseURL overrides the provider's default endpoint host. Needed for
	// self-hosted endpoints (Ollama) and to point tests at httptest servers.
	BaseURL string
	// HTTPClient is used for every request. nil means http.DefaultClient.
	HTTPClient *http.Client
	// PromptSeparator joins System and User for providers whose API takes a
	// single prompt string (Ollama). It exists to preserve the two different
	// joins the call sites used before this package; it disappears when the
	// vendor SDKs land (#33 step 3). Empty means "\n\n".
	PromptSeparator string
}

// factories is the provider registry: provider ID → client constructor.
var factories = map[string]func(Config) Client{
	ProviderOpenAI: newOpenAI,
	ProviderClaude: newAnthropic,
	ProviderOllama: newOllama,
}

// New returns a Client for the given provider ID.
func New(provider string, cfg Config) (Client, error) {
	factory, ok := factories[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %q", provider)
	}
	return factory(cfg), nil
}

// Providers returns the registered provider IDs, in no particular order.
func Providers() []string {
	ids := make([]string, 0, len(factories))
	for id := range factories {
		ids = append(ids, id)
	}
	return ids
}

// resolveBaseURL picks the configured host over the provider default and drops
// a trailing slash so endpoint paths can be appended directly.
func resolveBaseURL(configured, fallback string) string {
	if configured == "" {
		return fallback
	}
	return strings.TrimRight(configured, "/")
}

// postJSON marshals payload, POSTs it to url and returns the raw response body.
// Request and response bodies are logged at debug level through logger.Redact
// because they carry user text and credentials. name is the provider's display
// name and prefixes every error so callers can tell providers apart.
func postJSON(ctx context.Context, cfg Config, name, url string, headers map[string]string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%s marshal error: %w", name, err)
	}
	logger.Debug("llm: request", "provider", name, "url", url, "payload", logger.Redact(string(body)))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s build request: %w", name, err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", name, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s read response failed: %w", name, err)
	}
	logger.Debug("llm: response", "provider", name, "status", resp.StatusCode, "body", logger.Redact(string(respBody)))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s error %d: %s", name, resp.StatusCode, respBody)
	}
	return respBody, nil
}
