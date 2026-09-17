package llm

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestAnthropicCompleteRequestShape(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"content":[{"type":"text","text":"fixed text"}]}`)

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
	// The SDK sets this header now; the wire contract is unchanged.
	if h := got.headers.Get("anthropic-version"); h != "2023-06-01" {
		t.Errorf("anthropic-version = %q, want 2023-06-01", h)
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
	// The SDK sends system and message content as text blocks where the
	// hand-rolled client sent plain strings. Both are valid Messages API forms
	// and the model sees the same text; this pins the shape the SDK produces.
	if text := textBlocks(t, got.body["system"]); text != "system prompt" {
		t.Errorf("system = %v, want the system prompt", got.body["system"])
	}

	messages, ok := got.body["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages = %v, want one entry", got.body["messages"])
	}
	user := messages[0].(map[string]any)
	if user["role"] != "user" {
		t.Errorf("message role = %v, want user", user["role"])
	}
	if text := textBlocks(t, user["content"]); text != "user message" {
		t.Errorf("message content = %v, want the user message", user["content"])
	}
}

// textBlocks joins the text of an Anthropic content-block array.
func textBlocks(t *testing.T, value any) string {
	t.Helper()
	blocks, ok := value.([]any)
	if !ok {
		t.Fatalf("expected an array of content blocks, got %v", value)
	}
	var joined string
	for _, block := range blocks {
		entry, ok := block.(map[string]any)
		if !ok {
			t.Fatalf("expected a content block object, got %v", block)
		}
		if entry["type"] != "text" {
			t.Errorf("content block type = %v, want text", entry["type"])
		}
		text, _ := entry["text"].(string)
		joined += text
	}
	return joined
}

func TestAnthropicCompleteErrorStatus(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusTooManyRequests, `{"type":"error","error":{"type":"rate_limit_error","message":"slow down BODYMARKER"}}`)

	client := newAnthropic(Config{APIKey: "sk-ant-test", BaseURL: srv.URL})
	_, err := client.Complete(context.Background(), Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 4096})
	if err == nil {
		t.Fatal("expected an error for status 429")
	}
	if !strings.Contains(err.Error(), "Claude error 429") {
		t.Errorf("unexpected error: %v", err)
	}
	// The wording must be KeyLint's, not an echo of the provider's body.
	if strings.Contains(err.Error(), "BODYMARKER") {
		t.Errorf("the provider's body reached the error: %v", err)
	}
}

func TestAnthropicCompleteUnexpectedResponse(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"content":[]}`)

	client := newAnthropic(Config{APIKey: "sk-ant-test", BaseURL: srv.URL})
	_, err := client.Complete(context.Background(), Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 4096})
	if err == nil || !strings.Contains(err.Error(), "no text content") {
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
	srv := newServer(t, &got, http.StatusOK, `{"content":[{"type":"text","text":"ok"}]}`)

	client := newAnthropic(Config{APIKey: "sk-ant-test", BaseURL: srv.URL})
	req := Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 4096, JSONMode: true}
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if _, ok := got.body["response_format"]; ok {
		t.Error("response_format must not reach the Anthropic payload")
	}
}
