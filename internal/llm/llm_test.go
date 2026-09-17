package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"keylint/internal/logger"
)

// capture records the request a provider client sends and replies with a canned
// body, so tests can assert on the exact wire format.
type capture struct {
	// hits counts round trips, so a test can tell one request from a silent
	// retry — the SDKs retry internally and the log line looks the same.
	hits    int
	method  string
	path    string
	headers http.Header
	body    map[string]any
}

// newServer returns an httptest server that records the incoming request into
// got and answers with status and reply.
func newServer(t *testing.T, got *capture, status int, reply string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		got.hits++
		got.method = r.Method
		got.path = r.URL.Path
		got.headers = r.Header.Clone()
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &got.body); err != nil {
				t.Errorf("request body is not JSON: %v (%s)", err, raw)
			}
		}
		// Real providers answer as JSON, and the SDKs refuse anything else.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNewReturnsClientPerProvider(t *testing.T) {
	for _, provider := range []string{ProviderOpenAI, ProviderClaude, ProviderOllama} {
		client, err := New(provider, Config{})
		if err != nil {
			t.Fatalf("New(%q): %v", provider, err)
		}
		if client == nil {
			t.Fatalf("New(%q) returned nil client", provider)
		}
	}
}

func TestNewUnknownProvider(t *testing.T) {
	_, err := New("bedrock", Config{})
	if err == nil {
		t.Fatal("expected an error for an unregistered provider")
	}
	if !strings.Contains(err.Error(), `unsupported provider: "bedrock"`) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveBaseURL(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		want       string
	}{
		{"empty falls back", "", "https://fallback.test"},
		{"configured wins", "http://localhost:11434", "http://localhost:11434"},
		{"trailing slash trimmed", "http://localhost:11434/", "http://localhost:11434"},
		{"pasted whitespace trimmed", " http://localhost:11434 ", "http://localhost:11434"},
		{"whitespace only falls back", "   ", "https://fallback.test"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveBaseURL(tc.configured, "https://fallback.test"); got != tc.want {
				t.Errorf("resolveBaseURL(%q) = %q, want %q", tc.configured, got, tc.want)
			}
		})
	}
}

func TestCompleteCancelledContext(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"response":"never read"}`)

	client, err := New(ProviderOllama, Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.Complete(ctx, Request{Model: "llama3.2", User: "hi"})
	if err == nil {
		t.Fatal("expected an error for a cancelled context")
	}
	// Callers distinguish cancellation from a provider failure, so the wrapped
	// error has to stay unwrappable to context.Canceled.
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want one wrapping context.Canceled", err)
	}
}

func TestFallbackHTTPClientHasTimeout(t *testing.T) {
	// A nil Config.HTTPClient must not fall back to http.DefaultClient, which
	// never times out and would hang a request forever.
	if fallbackClient.Timeout == 0 {
		t.Error("fallbackClient has no timeout")
	}
	if fallbackClient == http.DefaultClient {
		t.Error("fallbackClient must not be http.DefaultClient")
	}
}

// TestProviderErrorBodyNeverLeaks is the regression test for #41. A provider's
// error body can echo the request — validation errors quote the offending
// field, content filters quote the text — so it must not end up in the error
// string, which gets formatted into Warn and Error log lines regardless of the
// sensitive-logging setting.
func TestProviderErrorBodyNeverLeaks(t *testing.T) {
	const marker = "SENSITIVE-USER-TEXT-cf83e1357eef"

	tests := []struct {
		provider string
		name     string
		cfg      Config
		req      Request
		body     string
	}{
		{
			provider: ProviderClaude,
			name:     "Claude",
			cfg:      Config{APIKey: "sk-ant-test"},
			req:      Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 4096},
			body:     `{"type":"error","error":{"type":"invalid_request_error","message":"prompt rejected: ` + marker + `"}}`,
		},
		{
			provider: ProviderOpenAI,
			name:     "OpenAI",
			cfg:      Config{APIKey: "sk-test"},
			req:      Request{Model: "gpt-4o-mini", User: "x"},
			body:     `{"error":{"type":"invalid_request_error","message":"prompt rejected: ` + marker + `"}}`,
		},
		{
			provider: ProviderOllama,
			name:     "Ollama",
			req:      Request{Model: "llama3.2", User: "x"},
			body:     `{"error":{"message":"prompt rejected: ` + marker + `"}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			// Debug level with sensitive logging off: the most revealing setting
			// a user can enable without opting into payload logging.
			var logs bytes.Buffer
			logger.InitWithWriter(&logs, "debug", false)
			// Restore the package default explicitly rather than through Init,
			// which would also reopen the real log file.
			t.Cleanup(func() { logger.InitWithWriter(io.Discard, "off", false) })

			var got capture
			srv := newServer(t, &got, http.StatusBadRequest, tc.body)

			cfg := tc.cfg
			cfg.BaseURL = srv.URL
			client, err := New(tc.provider, cfg)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			// The marker rides in on the request too: if logRequest lost its
			// Redact, the payload line would carry it.
			req := tc.req
			req.System = "system prompt containing " + marker
			req.User = "user text containing " + marker

			_, err = client.Complete(context.Background(), req)
			if err == nil {
				t.Fatal("expected an error for status 400")
			}

			if strings.Contains(err.Error(), marker) {
				t.Errorf("the provider's error body reached the error string: %v", err)
			}
			if strings.Contains(logs.String(), marker) {
				t.Errorf("the provider's error body reached the log:\n%s", logs.String())
			}

			// The toast still has to say something actionable.
			if !strings.Contains(err.Error(), tc.name) || !strings.Contains(err.Error(), "400") {
				t.Errorf("error = %v, want it to name the provider and the status", err)
			}
		})
	}
}

// TestSuccessfulResponseNeverLeaks is the other half of #41: the model's reply
// is user text too, and logResponse is the line that carries it.
func TestSuccessfulResponseNeverLeaks(t *testing.T) {
	const marker = "MODEL-OUTPUT-cf83e1357eef"

	var logs bytes.Buffer
	logger.InitWithWriter(&logs, "debug", false)
	t.Cleanup(func() { logger.InitWithWriter(io.Discard, "off", false) })

	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"choices":[{"message":{"content":"`+marker+`"}}]}`)

	client := newOpenAI(Config{APIKey: "sk-test", BaseURL: srv.URL})
	resp, err := client.Complete(context.Background(), Request{Model: "gpt-4o-mini", User: "x"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// The caller gets the text; the log does not.
	if resp.Text != marker {
		t.Errorf("Text = %q, want the model output", resp.Text)
	}
	if strings.Contains(logs.String(), marker) {
		t.Errorf("the model's reply reached the log:\n%s", logs.String())
	}
}

// TestErrorStatusSurvivesAStringErrorBody covers the OpenAI-compatible servers
// that answer {"error":"<string>"} instead of an object — llama.cpp, older
// Ollama, several proxies. The SDK cannot decode that into its typed error, and
// without the attempt middleware the user would get a JSON-unmarshal message
// with no HTTP status in it.
func TestErrorStatusSurvivesAStringErrorBody(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusNotFound, `{"error":"model 'llama3.2' not found"}`)

	client := newOllama(Config{BaseURL: srv.URL})
	_, err := client.Complete(context.Background(), Request{Model: "llama3.2", User: "x"})
	if err == nil {
		t.Fatal("expected an error for status 404")
	}
	if !strings.Contains(err.Error(), "Ollama error 404") {
		t.Errorf("error = %v, want it to name the status", err)
	}
	if strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("error = %v, want a user-facing message, not a decoder complaint", err)
	}
}

// TestRetryBudget pins how many round trips a user actually waits through. The
// SDK default is two retries; one keeps a transient blip covered without
// spending the caller's whole deadline on a rate limit that should have been
// reported straight away.
func TestRetryBudget(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusInternalServerError, `{"error":{"message":"boom"}}`)

	client := newOpenAI(Config{APIKey: "sk-test", BaseURL: srv.URL})
	if _, err := client.Complete(context.Background(), Request{Model: "gpt-4o-mini", User: "x"}); err == nil {
		t.Fatal("expected an error for status 500")
	}
	if want := sdkMaxRetries + 1; got.hits != want {
		t.Errorf("round trips = %d, want %d (one attempt plus %d retries)", got.hits, want, sdkMaxRetries)
	}
}

// TestOllamaBaseURLWithVersionSuffix covers what a user pastes: Ollama's own
// docs give the endpoint as .../v1, and appending another /v1 is a 404.
func TestOllamaBaseURLWithVersionSuffix(t *testing.T) {
	for _, suffix := range []string{"", "/", "/v1", "/v1/"} {
		t.Run("base"+suffix, func(t *testing.T) {
			var got capture
			srv := newServer(t, &got, http.StatusOK, `{"choices":[{"message":{"content":"ok"}}]}`)

			client := newOllama(Config{BaseURL: srv.URL + suffix})
			if _, err := client.Complete(context.Background(), Request{Model: "llama3.2", User: "x"}); err != nil {
				t.Fatalf("Complete: %v", err)
			}
			if got.path != "/v1/chat/completions" {
				t.Errorf("path = %q, want /v1/chat/completions", got.path)
			}
		})
	}
}

// TestNoMachineFingerprintOnTheWire pins what the SDKs are not allowed to tell a
// provider. Left alone both send the user's operating system, CPU architecture
// and Go runtime version on every call — including to a user-configured Ollama
// or OpenAI-compatible host, which is a fingerprint nobody opted into.
func TestNoMachineFingerprintOnTheWire(t *testing.T) {
	// A key in the environment must not reach a host the user pointed us at.
	t.Setenv("OPENAI_API_KEY", "sk-ENVIRONMENT-SHOULD-NOT-LEAK")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-ENVIRONMENT-SHOULD-NOT-LEAK")

	tests := []struct {
		provider string
		reply    string
		cfg      Config
		req      Request
		wantAuth string
	}{
		{
			provider: ProviderClaude,
			reply:    `{"content":[{"type":"text","text":"ok"}]}`,
			cfg:      Config{APIKey: "sk-ant-test"},
			req:      Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 16},
		},
		{
			provider: ProviderOpenAI,
			reply:    `{"choices":[{"message":{"content":"ok"}}]}`,
			cfg:      Config{APIKey: "sk-test"},
			req:      Request{Model: "gpt-4o-mini", User: "x"},
			wantAuth: "Bearer sk-test",
		},
		{
			provider: ProviderOllama,
			reply:    `{"choices":[{"message":{"content":"ok"}}]}`,
			req:      Request{Model: "llama3.2", User: "x"},
			// Ollama needs no credential; the placeholder must win over the
			// environment so a local daemon never sees somebody's OpenAI key.
			wantAuth: "Bearer " + ollamaAPIKey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			var got capture
			srv := newServer(t, &got, http.StatusOK, tc.reply)

			cfg := tc.cfg
			cfg.BaseURL = srv.URL
			client, err := New(tc.provider, cfg)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if _, err := client.Complete(context.Background(), tc.req); err != nil {
				t.Fatalf("Complete: %v", err)
			}

			for _, header := range fingerprintHeaders {
				if header == "User-Agent" {
					continue // replaced, not dropped — checked below
				}
				if value := got.headers.Get(header); value != "" {
					t.Errorf("%s = %q reached the provider", header, value)
				}
			}
			if ua := got.headers.Get("User-Agent"); ua != userAgent {
				t.Errorf("User-Agent = %q, want %q — a missing one becomes Go's default", ua, userAgent)
			}
			// Nothing may carry the machine's name for the SDK or its version.
			for name, values := range got.headers {
				for _, value := range values {
					if strings.Contains(value, runtime.GOOS) || strings.Contains(value, runtime.Version()) {
						t.Errorf("%s = %q leaks the operating system or Go version", name, value)
					}
				}
			}
			if tc.wantAuth != "" {
				if auth := got.headers.Get("Authorization"); auth != tc.wantAuth {
					t.Errorf("Authorization = %q, want %q", auth, tc.wantAuth)
				}
			}
			if key := got.headers.Get("x-api-key"); key != "" && strings.Contains(key, "ENVIRONMENT") {
				t.Errorf("x-api-key = %q came from the environment", key)
			}
		})
	}
}
