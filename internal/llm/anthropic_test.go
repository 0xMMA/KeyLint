package llm

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestAnthropicCompleteRequestShape(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"content":[{"text":"fixed text"}]}`)

	client := newAnthropic(Config{APIKey: "sk-ant-test", BaseURL: srv.URL})
	resp, err := client.Complete(context.Background(), Request{
		System:    "system prompt",
		User:      "user message",
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 2048,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "fixed text" {
		t.Errorf("Text = %q, want %q", resp.Text, "fixed text")
	}

	if got.method != http.MethodPost {
		t.Errorf("method = %q, want POST", got.method)
	}
	if got.path != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages", got.path)
	}
	if h := got.headers.Get("x-api-key"); h != "sk-ant-test" {
		t.Errorf("x-api-key = %q, want sk-ant-test", h)
	}
	if h := got.headers.Get("anthropic-version"); h != anthropicVersion {
		t.Errorf("anthropic-version = %q, want %q", h, anthropicVersion)
	}
	if h := got.headers.Get("Content-Type"); h != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", h)
	}
	if got.body["model"] != "claude-haiku-4-5-20251001" {
		t.Errorf("model = %v, want claude-haiku-4-5-20251001", got.body["model"])
	}
	if got.body["max_tokens"] != float64(2048) {
		t.Errorf("max_tokens = %v, want 2048", got.body["max_tokens"])
	}
	if got.body["system"] != "system prompt" {
		t.Errorf("system = %v, want the system prompt", got.body["system"])
	}

	messages, ok := got.body["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages = %v, want one entry", got.body["messages"])
	}
	user := messages[0].(map[string]any)
	if user["role"] != "user" || user["content"] != "user message" {
		t.Errorf("message = %v, want the user message", user)
	}
}

func TestAnthropicCompleteErrorStatus(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusTooManyRequests, `{"error":"rate limited"}`)

	client := newAnthropic(Config{APIKey: "sk-ant-test", BaseURL: srv.URL})
	_, err := client.Complete(context.Background(), Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 4096})
	if err == nil {
		t.Fatal("expected an error for status 429")
	}
	if !strings.Contains(err.Error(), "Claude error 429") || !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAnthropicCompleteUnexpectedResponse(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"content":[]}`)

	client := newAnthropic(Config{BaseURL: srv.URL})
	_, err := client.Complete(context.Background(), Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 4096})
	if err == nil || !strings.Contains(err.Error(), "Claude unexpected response") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAnthropicCompleteRequiresModelAndMaxTokens(t *testing.T) {
	client := newAnthropic(Config{})

	_, err := client.Complete(context.Background(), Request{User: "x", MaxTokens: 4096})
	if err == nil || !strings.Contains(err.Error(), "model is required") {
		t.Errorf("missing model: unexpected error: %v", err)
	}

	_, err = client.Complete(context.Background(), Request{User: "x", Model: "claude-sonnet-4-6"})
	if err == nil || !strings.Contains(err.Error(), "max_tokens is required") {
		t.Errorf("missing max_tokens: unexpected error: %v", err)
	}
}

// TestAnthropicIgnoresUnsupportedFields pins the wire format against #33 step 3:
// JSONMode has no Anthropic equivalent today and must not silently become one.
func TestAnthropicIgnoresUnsupportedFields(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"content":[{"text":"ok"}]}`)

	client := newAnthropic(Config{BaseURL: srv.URL})
	req := Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 4096, JSONMode: true}
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if _, ok := got.body["response_format"]; ok {
		t.Error("response_format must not reach the Anthropic payload")
	}
}
