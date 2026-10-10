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
	// Temperature pins the sampling temperature. nil leaves it to the provider,
	// which is what the product wants — a fix or a restructure reads better with
	// the provider's own default.
	//
	// It exists for measurement: an LLM-as-judge that scores the same output
	// differently on each run cannot tell a prompt change from noise. The Claude
	// Code CLI has no flag for it and ignores this field.
	Temperature *float64
	// Effort asks the model to think harder or less hard — one of the Effort*
	// levels. "" sends nothing, which leaves the model on its own default and is
	// what everyone gets who has not touched the setting.
	//
	// The Anthropic API takes it as output_config.effort and the Claude Code CLI
	// as --effort. Not every model accepts every level (Haiku 4.5 takes none,
	// the 4.6 generation no xhigh), and a level the model refuses is a 400 —
	// so the Anthropic client checks the account's model listing first and
	// leaves an unsupported level out, and both clients retry once without it
	// if the model rejects it anyway. OpenAI sends it as reasoning_effort, but
	// only to the model families whose support is documented (see
	// openAIReasoningModels); Ollama ignores it.
	//
	// It interacts with MaxTokens: thinking counts against the output limit, so
	// a high effort on a small limit (Fix's 2048) can end in the
	// outputLimitThinkingMessage cut-off.
	Effort string
	// NoThinking asks the model to answer without reasoning first, where the
	// model allows that. Fix sets it on a Haiku when no effort is chosen:
	// Haiku 5.5 reasons by default and spent all of Fix's 2048 tokens doing so
	// on a 3.8 KB selection (measured 2026-10-08), where Haiku 4.5 — the old
	// default — never reasoned. The Anthropic API takes it as
	// thinking: disabled, and OpenAI as reasoning_effort "none" on the models
	// documented to accept that; the Claude Code CLI has no flag for it and
	// Ollama ignores it. A model that cannot turn reasoning off (Sonnet 5.5,
	// Opus 5.5, GPT-6 Astra) is not sent it.
	NoThinking bool
}

// Effort levels, as both the Anthropic API and the Claude Code CLI name them.
const (
	EffortLow    = "low"
	EffortMedium = "medium"
	EffortHigh   = "high"
	EffortXHigh  = "xhigh"
	EffortMax    = "max"
)

// effortLevels in ascending order.
var effortLevels = []string{EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax}

// IsEffortLevel reports whether s is a level the providers know. "" is not a
// level: it is the absence of one.
func IsEffortLevel(s string) bool {
	for _, level := range effortLevels {
		if level == s {
			return true
		}
	}
	return false
}

// Temp is a helper for setting Request.Temperature, which is a pointer so that
// "not set" and "set to 0" are different things.
func Temp(v float64) *float64 { return &v }

// Response is the text of a completion.
type Response struct {
	Text string
	// Model is the model that answered, as the provider reports it — the
	// concrete ID behind an alias. Empty when the provider does not say. It is
	// what a measurement records, never the alias that was asked for.
	Model string
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
	// lastErr and lastTimedOut describe the previous attempt, so a retry can
	// be skipped when it cannot fit in what is left of the deadline.
	lastErr      error
	lastTimedOut bool
	// deadline is the caller's, taken from the context handed to Complete.
	// The request a middleware sees carries the SDK's per-attempt timeout
	// instead whenever that is the earlier one, so it cannot say how much of
	// the caller's time is left.
	deadline    time.Time
	hasDeadline bool
}

// newHTTPAttempts starts the record for one SDK call made under ctx.
func newHTTPAttempts(ctx context.Context, cfg Config, p provider) *httpAttempts {
	deadline, ok := ctx.Deadline()
	return &httpAttempts{cfg: cfg, provider: p, deadline: deadline, hasDeadline: ok}
}

// middleware fits both SDKs: their Middleware types are identical aliases.
func (a *httpAttempts) middleware(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	a.count++
	attempt := a.count

	// Redacted() masks any password in the URL — a self-hosted Ollama behind
	// basic auth is a realistic way for one to end up here.
	url := req.URL.Redacted()

	if attempt > 1 && !retryWorthwhile(a.lastTimedOut, a.hasDeadline, time.Until(a.deadline), a.cfg.requestTimeout()) {
		// Hand the SDK the previous attempt's timeout back: this is its last
		// attempt, so it returns that as the call's error and the user hears
		// "took too long" now rather than after another wait.
		logger.Info("llm: retry skipped, the time left cannot fit another attempt",
			"feature", a.cfg.Feature, "provider", a.provider.id, "attempt", attempt,
			"remaining_ms", time.Until(a.deadline).Milliseconds())
		// The attempt can have timed out with no error of its own: a response
		// arrived just as its context expired, and the SDK dropped it. A nil
		// error with a nil response would make the SDK read a status off nil.
		if a.lastErr == nil {
			return nil, context.DeadlineExceeded
		}
		return nil, a.lastErr
	}

	// Per attempt, not cumulative: a 500 followed by a dropped connection must
	// not let the retry be reported as "error 500".
	a.lastStatus = 0

	resp, err := next(req)
	a.lastErr = err
	a.lastTimedOut = isTimeoutCause(err) || errors.Is(req.Context().Err(), context.DeadlineExceeded)
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
		return &StatusError{Provider: p.name, Status: status, msg: fmt.Sprintf(
			"%s error %d: the model %q was not found, or the endpoint URL is wrong — check both in Settings → AI Providers",
			p.name, status, model)}
	}
	return &StatusError{Provider: p.name, Status: status, msg: fmt.Sprintf("%s error %d: %s", p.name, status, statusMessage(status))}
}

// rateLimitedPastDeadline is a rate limit the SDK was still waiting out when
// the caller's deadline passed: the provider answered 429 with a Retry-After
// longer than the time left. "Took too long" would send the user to a shorter
// selection; what they need to know is that they were rate limited. Still
// unwraps to the deadline, for callers that test for it.
func rateLimitedPastDeadline(p provider, attempts *httpAttempts, model string, err error) (error, bool) {
	if !errors.Is(err, context.DeadlineExceeded) || attempts.statusOrZero() != http.StatusTooManyRequests {
		return nil, false
	}
	se := apiErrorForModel(p, http.StatusTooManyRequests, model, "").(*StatusError)
	se.err = err
	return se, true
}

// transportError is the wording for a provider we could not reach at all.
//
// Never pass an SDK *apierror.Error here: its Error() concatenates the raw
// response body, which is exactly what #41 keeps out of error strings. Map it
// through apiError instead.
//
// A timeout gets its own wording (TimeoutError): "context deadline exceeded"
// is accurate and useless to the person waiting on it.
func transportError(p provider, err error) error {
	if isTimeoutCause(err) {
		return &TimeoutError{Provider: p.name, err: redactURLError(err)}
	}
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

// outputLimitThinkingMessage is the same cut-off when the model reasoned before
// answering — with no answer at all, or cut off mid-answer. It is kept apart
// because the cause differs, and an eval run records only the error string —
// the one generic message is how a thinking budget went unexplained in
// ADR-002. Telling a thinking model's user that the text is too long would be
// the same misdiagnosis; the Warn line's answer_bytes says which of the two
// shapes it was.
//
// It is what a Fix user on a thinking model (Sonnet 5, Opus 5) sees on a long
// selection, so it names the remedy. Worded for every surface it reaches — the
// GUI and the -fix / -pyramidize CLI — so it names the property that matters
// (the model reasons first) and not a settings tab.
//
// Lowering the effort is NOT named as the remedy: Haiku 5.5 at effort low
// still spent 2047 of Fix's 2048 tokens reasoning (measured 2026-10-08). What
// works is a Haiku with no effort set, which Fix asks not to reason at all.
const outputLimitThinkingMessage = "the model spent most of the output limit reasoning and was cut off — use a model that does not reason first (a Haiku with the effort on Model default), or shorten the text"

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
