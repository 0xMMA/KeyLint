// Package llm is the single place in KeyLint that knows how an LLM provider's
// HTTP API is shaped. Callers build a Request, pick a provider ID and get a
// Response back; features such as enhance and pyramidize must not contain
// provider-specific request or response handling.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"keylint/internal/logger"
)

// Provider IDs. These match the values persisted in settings (ActiveProvider).
const (
	ProviderOpenAI = "openai"
	ProviderClaude = "claude"
	ProviderOllama = "ollama"
	// ProviderClaudeCode runs the Claude Code CLI the user installed and signed
	// into themselves. KeyLint spawns the unmodified binary and never reads,
	// stores or forwards their credentials.
	ProviderClaudeCode = "claude-code"
)

// provider pairs the stable ID used in settings and logs with the display name
// that appears in errors the user reads.
type provider struct {
	id   string
	name string
}

// Request is a single-turn completion request.
type Request struct {
	// System is the system prompt. Every provider now has a real system role,
	// including Ollama through its OpenAI-compatible endpoint.
	System string
	// User is the user message.
	User string
	// Model is the provider-specific model ID and is required — defaults live
	// at the call site until they move into settings (#33 step 4).
	Model string
	// MaxTokens caps the response length. Only providers whose API requires it
	// (Anthropic) send it; the others keep the request shape they had before.
	MaxTokens int
	// JSONMode asks the provider to constrain output to a JSON object. OpenAI
	// and the OpenAI-compatible Ollama endpoint enforce it; Anthropic and the
	// Claude Code CLI have no equivalent today, so their callers parse
	// defensively. Widening this to a JSON schema is #47.
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
	// HTTPClient is used for every request. nil falls back to a client with
	// fallbackTimeout — never http.DefaultClient, which would never time out.
	HTTPClient *http.Client
	// Feature names the caller ("enhance", "pyramidize") and appears in the logs
	// so a user's log file still says which flow a request came from. It is not
	// called Source because the logger already owns that key for backend vs
	// frontend, and two attributes with one name make a log line ambiguous.
	Feature string
	// CLIPath points at a provider that is a local executable rather than an
	// HTTP endpoint (Claude Code). Empty means "find it on this machine"; tests
	// set it to a stub binary.
	CLIPath string
}

// factories is the provider registry: provider ID → client constructor. It is
// written at package init only; adding runtime registration would need a lock.
var factories = map[string]func(Config) Client{
	ProviderOpenAI:     newOpenAI,
	ProviderClaude:     newAnthropic,
	ProviderOllama:     newOllama,
	ProviderClaudeCode: newClaudeCode,
}

// fallbackTimeout bounds requests when the caller passes no HTTP client.
const fallbackTimeout = 90 * time.Second

// fallbackClient is used when Config.HTTPClient is nil.
var fallbackClient = &http.Client{Timeout: fallbackTimeout}

// New returns a Client for the given provider ID.
func New(provider string, cfg Config) (Client, error) {
	factory, ok := factories[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %q", provider)
	}
	return factory(cfg), nil
}

// resolveBaseURL picks the configured host over the provider default and drops
// a trailing slash so endpoint paths can be appended directly.
func resolveBaseURL(configured, fallback string) string {
	// A user types the Ollama URL into a free-text field, so a stray space or
	// trailing slash is a paste artefact, not a different endpoint.
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return fallback
	}
	return strings.TrimRight(configured, "/")
}

// httpClient returns the client to hand the vendor SDKs. Ours carries the
// timeout; the SDKs add their own retries on top of it.
func (c Config) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return fallbackClient
}

// httpAttempts records the round trips the SDK makes. The SDKs retry
// internally, so without this a log shows one request and one response even
// when three round trips happened — and a failure the SDK cannot decode into
// its typed error loses the HTTP status it came with.
type httpAttempts struct {
	cfg        Config
	provider   provider
	count      int
	lastStatus int
}

// middleware fits both SDKs: their Middleware types are identical aliases.
func (a *httpAttempts) middleware(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	a.count++
	attempt := a.count

	resp, err := next(req)
	if err != nil {
		logger.Debug("llm: http", "feature", a.cfg.Feature, "provider", a.provider.id,
			"method", req.Method, "url", req.URL.String(), "attempt", attempt, "err", err)
		return resp, err
	}
	a.lastStatus = resp.StatusCode
	logger.Debug("llm: http", "feature", a.cfg.Feature, "provider", a.provider.id,
		"method", req.Method, "url", req.URL.String(), "attempt", attempt, "status", resp.StatusCode)
	return resp, nil
}

// statusOrZero is the status of the last response, or 0 if none arrived.
func (a *httpAttempts) statusOrZero() int { return a.lastStatus }

// logRequest records what we are about to send. The payload is user text, so it
// only ever reaches the log through Redact.
func logRequest(cfg Config, p provider, model string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		body = []byte(fmt.Sprintf("<unserialisable: %v>", err))
	}
	logger.Debug("llm: request", "feature", cfg.Feature, "provider", p.id,
		"model", model, "payload", logger.Redact(string(body)))
}

// logResponse records what came back, again only through Redact.
func logResponse(cfg Config, p provider, text string) {
	logger.Debug("llm: response", "feature", cfg.Feature, "provider", p.id,
		"text", logger.Redact(text))
}

// statusMessage is the one-line reason a user sees, keyed by HTTP status.
//
// The provider's own error message is deliberately not used (#41): validation
// and content-filter messages echo parts of the request, so putting one in an
// error string leaks user text into every log line that formats that error,
// whatever the sensitive-logging setting says. The raw body goes to Debug
// through Redact and nowhere else.
func statusMessage(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "the request was rejected as invalid"
	case http.StatusPaymentRequired:
		// The one status whose cause is unambiguous and not sensitive.
		return "the account is out of credit or has a billing problem"
	case http.StatusUnauthorized:
		return "the API key was not accepted"
	case http.StatusForbidden:
		return "this key is not allowed to use that model"
	case http.StatusNotFound:
		return "the model or endpoint was not found"
	case http.StatusRequestTimeout:
		return "the provider timed out"
	case http.StatusRequestEntityTooLarge:
		return "the request was too large"
	case http.StatusTooManyRequests:
		return "rate limited — try again shortly"
	}
	if status >= 500 {
		return "the provider is unavailable"
	}
	return "the provider rejected the request"
}

// apiError is the user-facing wording for a provider that answered with a
// status: "<Provider> error <status>: <reason>".
func apiError(p provider, status int, rawBody string) error {
	if rawBody != "" {
		logger.Debug("llm: error body", "provider", p.id, "status", status, "body", logger.Redact(rawBody))
	}
	return fmt.Errorf("%s error %d: %s", p.name, status, statusMessage(status))
}

// transportError is the wording for a provider we could not reach at all.
//
// Never pass an SDK *apierror.Error here: its Error() concatenates the raw
// response body, which is exactly what #41 keeps out of error strings. Map it
// through apiError instead.
func transportError(p provider, err error) error {
	return fmt.Errorf("%s request failed: %w", p.name, err)
}

// sdkOptions are the settings both vendor SDKs get.
//
// MaxRetries is 1, not the SDK default of 2. The retry loop honours the
// context, so a retried 429 with a long Retry-After spends the caller's whole
// budget and then surfaces as "context deadline exceeded" — the user waits
// silently and never learns they were rate limited. One retry keeps the
// benefit for a transient blip without hiding a real one for two minutes.
const sdkMaxRetries = 1

// requestTimeout is the per-attempt budget handed to the SDK, so the timeout it
// advertises to the server matches the one our HTTP client enforces.
func (c Config) requestTimeout() time.Duration {
	return c.httpClient().Timeout
}
