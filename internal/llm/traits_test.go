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
		ProviderOpenRouter: true,
		ProviderClaudeCode: false,
		ProviderOllama:     false,
		"":                 false,
	} {
		// The Bedrock mode means nothing to any other provider.
		for _, mode := range []string{"", BedrockAuthKey, BedrockAuthSignIn} {
			if got := UsesAPIKey(provider, mode); got != want {
				t.Errorf("UsesAPIKey(%q, %q) = %v, want %v", provider, mode, got, want)
			}
		}
	}
}

// TestUsesAPIKeyFollowsTheBedrockMode: a Bedrock key in key mode, none when
// the computer's AWS sign-in is used. Key mode is the default, so a settings
// file that names no mode asks for a key rather than reading ~/.aws.
func TestUsesAPIKeyFollowsTheBedrockMode(t *testing.T) {
	for mode, want := range map[string]bool{
		"":                true,
		BedrockAuthKey:    true,
		BedrockAuthSignIn: false,
		"something-else":  true,
	} {
		if got := UsesAPIKey(ProviderBedrock, mode); got != want {
			t.Errorf("UsesAPIKey(bedrock, %q) = %v, want %v", mode, got, want)
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
		// Bedrock serves the same models under its own IDs, with or without
		// a routing prefix.
		{ProviderBedrock, "anthropic.claude-haiku-4-5-20251001-v1:0", true},
		{ProviderBedrock, "eu.anthropic.claude-haiku-4-5-20251001-v1:0", true},
		{ProviderBedrock, "anthropic.claude-haiku-5-5", false},
		{ProviderBedrock, "global.anthropic.claude-sonnet-5-5", false},
		{ProviderBedrock, "anthropic.claude-opus-4-8", false},
		{ProviderBedrock, "arn:aws:bedrock:eu-central-1:123456789012:application-inference-profile/abc", false},
		{ProviderOpenAI, "gpt-4.1", true},
		{ProviderOllama, "llama3", true},
	}
	for _, tc := range cases {
		if got := SupportsTemperature(tc.provider, tc.model); got != tc.want {
			t.Errorf("SupportsTemperature(%s, %s) = %v, want %v", tc.provider, tc.model, got, tc.want)
		}
	}
}

// TestIsFastTier: Fix asks a fast-tier model to answer without reasoning
// first. OpenRouter names the same models with the vendor in front
// (anthropic/claude-haiku-5.5) and keeps "latest" IDs of its own under a "~";
// missing them would let Haiku 5.5 spend Fix's 2048 tokens reasoning there, as
// it did on the Anthropic API. A Pro variant is the same model with more
// compute — what someone picks it for is the reasoning.
func TestIsFastTier(t *testing.T) {
	fast := []string{
		"haiku", "claude-haiku-5-5", "claude-haiku-4-5-20251001", "gpt-6-luna", "gpt-5.6-luna",
		"anthropic/claude-haiku-5.5", "anthropic/claude-haiku-4.5", "~anthropic/claude-haiku-latest",
		"openai/gpt-6-luna", "openai/gpt-5.6-luna", "~openai/gpt-luna-latest",
	}
	notFast := []string{
		"sonnet", "claude-sonnet-5-5", "gpt-6-astra", "",
		"anthropic/claude-sonnet-5.5", "anthropic/claude-opus-5.5", "openai/gpt-6-astra",
		"openai/gpt-6-luna-pro", "openai/gpt-5.6-luna-pro", "gpt-6-luna-pro",
		// Only the vendor's own models: another host's ID that happens to
		// contain the name is not known to be the same model.
		"somehost/claude-haiku-5.5", "google/gemini-3.8-flash",
	}
	for _, m := range fast {
		if !IsFastTier(m) {
			t.Errorf("IsFastTier(%q) = false, want true", m)
		}
	}
	for _, m := range notFast {
		if IsFastTier(m) {
			t.Errorf("IsFastTier(%q) = true, want false", m)
		}
	}
}
