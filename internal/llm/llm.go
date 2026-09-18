// Package llm is the single place in KeyLint that knows how an LLM provider's
// HTTP API is shaped. Callers build a Request, pick a provider ID and get a
// Response back; features such as enhance and pyramidize must not contain
// provider-specific request or response handling.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
	// Model is the provider-specific model ID and is required. Callers resolve
	// it through settings and llm.DefaultModel; this package does not guess.
	Model string
	// MaxTokens caps the response length. Only providers whose API requires it
	// (Anthropic) send it; the others keep the request shape they had before.
	MaxTokens int
	// JSONMode asks for a JSON object without saying what shape. It is the
	// weaker constraint, used when a caller parses JSON but schema enforcement
	// is off; JSONSchema wins where both are set. OpenAI and the
	// OpenAI-compatible Ollama endpoint support it, the others ignore it.
	JSONMode bool
	// JSONSchema constrains the reply to a shape. nil means no constraint.
	//
	// Every provider enforces it in its own dialect — OpenAI and the
	// OpenAI-compatible Ollama endpoint through response_format, Anthropic
	// through output_config, the Claude Code CLI through --json-schema — but
	// none of them is a parser: a caller still unmarshals the text it gets back,
	// and should still do so defensively.
	JSONSchema json.RawMessage
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
	// Per attempt, not cumulative: a 500 followed by a dropped connection must
	// not let the retry be reported as "error 500".
	a.lastStatus = 0

	// Redacted() masks any password in the URL — a self-hosted Ollama behind
	// basic auth is a realistic way for one to end up here.
	url := req.URL.Redacted()

	resp, err := next(req)
	if err != nil {
		logger.Debug("llm: http", "feature", a.cfg.Feature, "provider", a.provider.id,
			"method", req.Method, "url", url, "attempt", attempt, "err", err)
		return resp, err
	}
	a.lastStatus = resp.StatusCode
	logger.Debug("llm: http", "feature", a.cfg.Feature, "provider", a.provider.id,
		"method", req.Method, "url", url, "attempt", attempt, "status", resp.StatusCode)
	return resp, nil
}

// statusOrZero is the status of the last attempt's response, or 0 if none
// arrived.
func (a *httpAttempts) statusOrZero() int { return a.lastStatus }

// isContextError reports whether the caller gave up rather than the provider
// failing. Callers test for this with errors.Is, so it must never be reported
// as a provider status.
func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

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
	return apiErrorForModel(p, status, "", rawBody)
}

// apiErrorForModel is apiError for a call that named a model. A 404 then almost
// always means the configured model is gone or was never available to this
// account, and an error that does not say which one leaves the user hunting.
func apiErrorForModel(p provider, status int, model, rawBody string) error {
	if rawBody != "" {
		logger.Debug("llm: error body", "provider", p.id, "status", status, "body", logger.Redact(rawBody))
	}
	if status == http.StatusNotFound && model != "" {
		// Naming the model matters — a model configured months ago and since
		// retired is the likeliest 404 here — but it must not displace the other
		// cause the old wording carried: a mistyped endpoint answers 404 too,
		// and sending that user to the model picker wastes their time.
		return fmt.Errorf("%s error %d: the model %q was not found, or the endpoint URL is wrong — check both in Settings → AI Providers",
			p.name, status, model)
	}
	return fmt.Errorf("%s error %d: %s", p.name, status, statusMessage(status))
}

// transportError is the wording for a provider we could not reach at all.
//
// Never pass an SDK *apierror.Error here: its Error() concatenates the raw
// response body, which is exactly what #41 keeps out of error strings. Map it
// through apiError instead.
func transportError(p provider, err error) error {
	return fmt.Errorf("%s request failed: %w", p.name, redactURLError(err))
}

// redactURLError strips userinfo from the URL a *url.Error carries.
//
// Go puts the whole request URL in the message, and a self-hosted Ollama or an
// OpenAI-compatible gateway is reached through a URL the user typed — which may
// carry credentials. Those errors are logged at Info and shown in the UI, so
// they must not repeat the secret back.
func redactURLError(err error) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}
	parsed, perr := url.Parse(urlErr.URL)
	if perr != nil || parsed.User == nil {
		return err
	}
	parsed.User = nil
	cleaned := *urlErr
	cleaned.URL = parsed.String()
	return &cleaned
}

// fingerprintHeaders describe the machine rather than the request. Both SDKs
// send them by default: the operating system, CPU architecture and Go runtime
// version of whoever is typing, on every hotkey fix. KeyLint has no use for
// that reaching a provider, and for a user-configured Ollama or
// OpenAI-compatible host it would be a fingerprint they never opted into.
// Deleting them leaves the retry loop untouched — only the attempt number stops
// being reported to the server.
//
// User-Agent is replaced rather than dropped: net/http substitutes its own
// "Go-http-client/1.1" for a missing one, which some gateways treat as a bot,
// and userAgent says who is calling without saying anything about the machine.
var fingerprintHeaders = []string{
	"User-Agent",
	"X-Stainless-Arch",
	"X-Stainless-Lang",
	"X-Stainless-OS",
	"X-Stainless-Package-Version",
	"X-Stainless-Retry-Count",
	"X-Stainless-Runtime",
	"X-Stainless-Runtime-Version",
	"X-Stainless-Timeout",
}

// outputLimitMessage is what a user reads when a reply was cut off. The Fix
// page caps at 2048 tokens, so this is reachable on ordinary long selections
// and has to say what to do about it rather than name a limit.
const outputLimitMessage = "the result exceeded the output limit — try a shorter selection"

// schemaName labels the schema for providers that require a name for it. It is
// never shown to a user.
const schemaName = "keylint_result"

// schemaObject decodes a caller's schema for the SDKs that want a map rather
// than raw bytes.
func schemaObject(schema json.RawMessage) (map[string]any, error) {
	var object map[string]any
	if err := json.Unmarshal(schema, &object); err != nil {
		return nil, fmt.Errorf("invalid JSON schema: %w", err)
	}
	return object, nil
}

// userAgent identifies the application, deliberately without a version: the
// provider has no need to know which build of KeyLint a user is running.
const userAgent = "KeyLint"

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
