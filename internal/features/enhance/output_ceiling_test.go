package enhance

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
)

// TestFixOutputCeilingReachesTheWire sends a fix through the real Anthropic
// client to a local server and reads max_tokens off the request body.
//
// Sonnet 5 and Opus 5 think by default, and thinking counts against
// max_tokens. Measured with Sonnet 5 on a 3.7 KB selection, the old 2048 was
// spent entirely on thinking twice out of two — stop_reason max_tokens, not
// one visible character — while the same request at 16000 finished with 5361
// output tokens. The user saw "try a shorter selection" for text that was not
// too long.
func TestFixOutputCeilingReachesTheWire(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("request body is not JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"stop_reason":"end_turn","content":[{"type":"text","text":"Fixed."}]}`)
	}))
	t.Cleanup(srv.Close)

	cfg := settings.Default()
	cfg.ActiveProvider = llm.ProviderClaude
	cfg.Models = map[string]settings.FeatureModels{llm.ProviderClaude: {Fix: "claude-sonnet-5"}}
	svc, _ := newTestService(t, cfg, map[string]string{llm.ProviderClaude: "sk-ant-test"})
	svc.newClient = func(provider string, clientCfg llm.Config) (llm.Client, error) {
		clientCfg.BaseURL = srv.URL
		return llm.New(provider, clientCfg)
	}

	if _, err := svc.Enhance("text"); err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if body["model"] != "claude-sonnet-5" {
		t.Fatalf("model = %v, want the configured claude-sonnet-5", body["model"])
	}
	if body["max_tokens"] != float64(16000) {
		t.Errorf("max_tokens = %v, want 16000", body["max_tokens"])
	}
	// Headroom only: thinking and effort stay at the model's default, which is
	// an eval-gated quality decision (E3, #34) and not this change's to make.
	if _, ok := body["thinking"]; ok {
		t.Errorf("thinking = %v, want it absent so the model default applies", body["thinking"])
	}
}
