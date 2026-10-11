package llm

import (
	"context"
	"net/http"
	"testing"
)

func TestUsesAPIKey(t *testing.T) {
	for provider, want := range map[string]bool{
		ProviderClaude:     true,
		ProviderOpenAI:     true,
		ProviderClaudeCode: false,
		ProviderOllama:     false,
		"":                 false,
	} {
		if got := UsesAPIKey(provider); got != want {
			t.Errorf("UsesAPIKey(%q) = %v, want %v", provider, got, want)
		}
	}
}

// The HTTP providers report the model in their response body; Response.Model
// must carry it through rather than echoing the request.
func TestAnthropicReportsTheModelThatAnswered(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK,
		`{"model":"claude-sonnet-4-6-20260101","content":[{"type":"text","text":"ok"}]}`)
	resp, err := newAnthropic(Config{APIKey: "k", BaseURL: srv.URL}).Complete(context.Background(),
		Request{User: "x", Model: "claude-sonnet-4-6", MaxTokens: 10})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Model != "claude-sonnet-4-6-20260101" {
		t.Errorf("Model = %q, want the ID from the response body", resp.Model)
	}
}

func TestOpenAIReportsTheModelThatAnswered(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK,
		`{"model":"gpt-4o-mini-2024-07-18","choices":[{"message":{"content":"ok"}}]}`)
	resp, err := newOpenAI(Config{APIKey: "k", BaseURL: srv.URL}).Complete(context.Background(),
		Request{User: "x", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Model != "gpt-4o-mini-2024-07-18" {
		t.Errorf("Model = %q, want the ID from the response body", resp.Model)
	}
}

func TestSupportsTemperature(t *testing.T) {
	if SupportsTemperature(ProviderClaudeCode) {
		t.Error("the Claude Code CLI has no temperature flag; claiming otherwise records a pin that never happened")
	}
	for _, p := range []string{ProviderClaude, ProviderOpenAI, ProviderOllama} {
		if !SupportsTemperature(p) {
			t.Errorf("SupportsTemperature(%q) = false, want true", p)
		}
	}
}
