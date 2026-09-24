package pyramidize

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
)

// TestPyramidizeOutputCeilingReachesTheWire sends a Pyramidize step through
// the real Anthropic client to a local server and reads max_tokens off the
// request body — the number the API actually enforces.
//
// Sonnet 5 and Opus 5 think by default when the request says nothing about
// thinking, and every thinking token counts against max_tokens. Measured on
// the two largest eval samples, a Pyramidize reply spent 2352–3974 output
// tokens of which the visible document was roughly 700–1250: at the old 4096
// the answer was a few hundred tokens from being cut off, and at 2048 it was
// cut off before a single visible token. 16000 is headroom, not spend.
func TestPyramidizeOutputCeilingReachesTheWire(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("request body is not JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"stop_reason":"end_turn","content":[{"type":"text","text":"{\"ok\":true}"}]}`)
	}))
	t.Cleanup(srv.Close)

	svc := &Service{client: srv.Client(), newClient: func(provider string, cfg llm.Config) (llm.Client, error) {
		cfg.BaseURL = srv.URL
		return llm.New(provider, cfg)
	}}
	cfg := settings.Default()
	cfg.ActiveProvider = llm.ProviderClaude

	opts := aiOpts{model: "claude-sonnet-5"}
	if _, err := svc.callAISync(context.Background(), cfg, opts, "sk-ant-test", "system", "user", nil); err != nil {
		t.Fatalf("callAISync: %v", err)
	}

	if body["max_tokens"] != float64(16000) {
		t.Errorf("max_tokens = %v, want 16000", body["max_tokens"])
	}
	// The fix is headroom only. Whether and how hard the model thinks is a
	// quality decision that belongs to an eval run (E3, #34), not to this
	// change — so the request must still leave thinking and effort unset.
	if _, ok := body["thinking"]; ok {
		t.Errorf("thinking = %v, want it absent so the model default applies", body["thinking"])
	}
	if _, ok := body["output_config"]; ok {
		t.Errorf("output_config = %v, want it absent: no schema was asked for and effort is not set here", body["output_config"])
	}
}
