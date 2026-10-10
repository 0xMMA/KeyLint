package silentfix

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"keylint/internal/llm"
)

// newStatusServer answers every request with status, as the Anthropic API
// would, and asks the SDK not to retry so the test does not wait on backoff.
func newStatusServer(t *testing.T, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-should-retry", "false")
		w.WriteHeader(status)
		fmt.Fprint(w, `{"type":"error","error":{"type":"error","message":"x"}}`)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func mustClient(t *testing.T, baseURL string) llm.Client {
	t.Helper()
	client, err := llm.New(llm.ProviderClaude, llm.Config{APIKey: "sk-ant-test", BaseURL: baseURL})
	if err != nil {
		t.Fatalf("llm.New: %v", err)
	}
	return client
}
