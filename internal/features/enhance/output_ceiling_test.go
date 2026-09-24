package enhance

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
)

// fixThroughAnthropic runs Enhance through the real Anthropic client against a
// local server that answers with reply, and returns the request body it got.
func fixThroughAnthropic(t *testing.T, reply string) (map[string]any, error) {
	t.Helper()
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("request body is not JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, reply)
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
	_, err := svc.Enhance("text")
	return body, err
}

// TestFixKeepsItsOutputLimit pins the silent Fix at 2048 on the wire, on a
// model that thinks by default. Pyramidize raised its limit; Fix did not, on
// purpose — see maxTokens. Moving this number is a product decision.
func TestFixKeepsItsOutputLimit(t *testing.T) {
	body, err := fixThroughAnthropic(t, `{"stop_reason":"end_turn","content":[{"type":"text","text":"Fixed."}]}`)
	if err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if body["model"] != "claude-sonnet-5" {
		t.Fatalf("model = %v, want the configured claude-sonnet-5", body["model"])
	}
	if body["max_tokens"] != float64(2048) {
		t.Errorf("max_tokens = %v, want 2048", body["max_tokens"])
	}
	// Thinking and effort stay at the model's default: an eval-gated quality
	// decision (E3, #34), not this limit's to make.
	if _, ok := body["thinking"]; ok {
		t.Errorf("thinking = %v, want it absent so the model default applies", body["thinking"])
	}
	if _, ok := body["output_config"]; ok {
		t.Errorf("output_config = %v, want it absent: Fix asks for prose and sets no effort", body["output_config"])
	}
}

// TestFixOnAThinkingModelSaysWhatToDo is what a Sonnet 5 user sees on a long
// selection: measured, the whole 2048 goes on thinking and no text comes back.
// The error must name a remedy, not only the limit.
func TestFixOnAThinkingModelSaysWhatToDo(t *testing.T) {
	_, err := fixThroughAnthropic(t,
		`{"stop_reason":"max_tokens","usage":{"input_tokens":2878,"output_tokens":2048},"content":[{"type":"thinking","thinking":"","signature":"s"}]}`)
	if err == nil {
		t.Fatal("expected an error for a reply with no text")
	}
	for _, want := range []string{"reasoning", "pick a faster model", "Settings", "shorten the text"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to mention %q", err, want)
		}
	}
}
