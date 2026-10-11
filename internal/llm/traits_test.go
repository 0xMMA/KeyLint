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

// TestSupportsTemperature: the CLI has no temperature flag at all, and on the
// Anthropic API the 4.7 generation and everything after it rejects a
// non-default temperature (Sonnet 5.5 and Haiku 5.5 with a 400, Opus 4.7/4.8,
// Sonnet 5, Opus 5.x and Fable have no sampling parameters at all). Claiming
// a pin there records one that never reached the model — or fails the call.
func TestSupportsTemperature(t *testing.T) {
	cases := []struct {
		provider, model string
		want            bool
	}{
		{ProviderClaudeCode, "claude-sonnet-4-5-20250929", false},
		{ProviderClaudeCode, "haiku", false},
		{ProviderClaude, "claude-sonnet-4-5-20250929", true},
		{ProviderClaude, "claude-haiku-4-5-20251001", true},
		{ProviderClaude, "claude-sonnet-4-6", true},
		{ProviderClaude, "claude-opus-4-6", true},
		{ProviderClaude, "claude-3-5-sonnet-20241022", true},
		{ProviderClaude, "claude-opus-4-7", false},
		{ProviderClaude, "claude-opus-4-8", false},
		{ProviderClaude, "claude-sonnet-5", false},
		{ProviderClaude, "claude-sonnet-5-5", false},
		{ProviderClaude, "claude-haiku-5-5", false},
		{ProviderClaude, "claude-opus-5-5", false},
		{ProviderClaude, "claude-fable-5-1", false},
		// A family alias resolves to the newest generation.
		{ProviderClaude, "sonnet", false},
		{ProviderClaude, "something-else", false},
		{ProviderOpenAI, "gpt-4.1", true},
		{ProviderOllama, "llama3", true},
	}
	for _, tc := range cases {
		if got := SupportsTemperature(tc.provider, tc.model); got != tc.want {
			t.Errorf("SupportsTemperature(%s, %s) = %v, want %v", tc.provider, tc.model, got, tc.want)
		}
	}
}
