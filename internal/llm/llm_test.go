package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// capture records the request a provider client sends and replies with a canned
// body, so tests can assert on the exact wire format.
type capture struct {
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
		got.method = r.Method
		got.path = r.URL.Path
		got.headers = r.Header.Clone()
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &got.body); err != nil {
				t.Errorf("request body is not JSON: %v (%s)", err, raw)
			}
		}
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
