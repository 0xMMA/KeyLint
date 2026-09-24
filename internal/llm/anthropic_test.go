package llm

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"keylint/internal/logger"
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

// TestAnthropicJSONSchema pins the Anthropic dialect: a schema travels in
// output_config.format, not in OpenAI's response_format.
func TestAnthropicJSONSchema(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"content":[{"type":"text","text":"{}"}]}`)

	client := newAnthropic(Config{APIKey: "sk-ant-test", BaseURL: srv.URL})
	req := Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 4096, JSONSchema: []byte(testSchema)}
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if _, ok := got.body["response_format"]; ok {
		t.Error("response_format is OpenAI's field and must not reach Anthropic")
	}
	config, ok := got.body["output_config"].(map[string]any)
	if !ok {
		t.Fatalf("output_config = %v, want an object", got.body["output_config"])
	}
	format, ok := config["format"].(map[string]any)
	if !ok {
		t.Fatalf("output_config.format = %v, want an object", config["format"])
	}
	if format["type"] != "json_schema" {
		t.Errorf("output_config.format.type = %v, want json_schema", format["type"])
	}
	schema, ok := format["schema"].(map[string]any)
	if !ok {
		t.Fatalf("output_config.format.schema = %v, want the caller's schema", format["schema"])
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok || properties["answer"] == nil {
		t.Errorf("schema did not survive: %v", schema)
	}
}

func TestAnthropicWithoutSchema(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"content":[{"type":"text","text":"ok"}]}`)

	client := newAnthropic(Config{APIKey: "sk-ant-test", BaseURL: srv.URL})
	req := Request{Model: "claude-haiku-4-5-20251001", User: "x", MaxTokens: 2048}
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// enhance sends no schema and must not be constrained.
	if _, ok := got.body["output_config"]; ok {
		t.Error("output_config must be absent when the caller set no schema")
	}
}

// TestAnthropicRefusesUnusableAnswers mirrors the OpenAI cases: a cut-off or
// declined answer must not reach the caller as text.
func TestAnthropicRefusesUnusableAnswers(t *testing.T) {
	tests := []struct {
		name  string
		reply string
		want  string
	}{
		{
			name:  "cut off at the output limit",
			reply: `{"stop_reason":"max_tokens","content":[{"type":"text","text":"They are going to the"}]}`,
			want:  "exceeded the output limit",
		},
		{
			// Sonnet 5 and Opus 5 think by default. With display omitted the
			// thinking block arrives empty, and a budget spent on it leaves no
			// text at all — the shape measured on Sonnet 5 at 2048 tokens.
			name:  "output limit spent on thinking before any answer",
			reply: `{"stop_reason":"max_tokens","content":[{"type":"thinking","thinking":"","signature":"sig"}]}`,
			want:  "used the whole output limit reasoning before it answered",
		},
		{
			name:  "output limit spent on redacted thinking",
			reply: `{"stop_reason":"max_tokens","content":[{"type":"redacted_thinking","data":"opaque"}]}`,
			want:  "used the whole output limit reasoning before it answered",
		},
		{
			// Thinking and a partial answer: still an ordinary cut-off.
			name:  "cut off mid-answer after thinking",
			reply: `{"stop_reason":"max_tokens","content":[{"type":"thinking","thinking":"","signature":"sig"},{"type":"text","text":"They are going"}]}`,
			want:  "exceeded the output limit",
		},
		{
			name:  "context window exceeded",
			reply: `{"stop_reason":"model_context_window_exceeded","content":[{"type":"text","text":"partial"}]}`,
			want:  "exceeded the output limit",
		},
		{
			name:  "model declined",
			reply: `{"stop_reason":"refusal","content":[{"type":"text","text":""}]}`,
			want:  "declined the request",
		},
		{
			name:  "empty content",
			reply: `{"stop_reason":"end_turn","content":[{"type":"text","text":"   "}]}`,
			want:  "no text content",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got capture
			srv := newServer(t, &got, http.StatusOK, tc.reply)

			client := newAnthropic(Config{APIKey: "sk-ant-test", BaseURL: srv.URL})
			req := Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 2048}
			_, err := client.Complete(context.Background(), req)
			if err == nil {
				t.Fatal("expected an error rather than an unusable answer")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// TestAnthropicJoinsTextBlocks: a reply can arrive in several blocks, and taking
// only the first would truncate it silently.
func TestAnthropicJoinsTextBlocks(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK,
		`{"stop_reason":"end_turn","content":[{"type":"text","text":"first "},{"type":"text","text":"second"}]}`)

	client := newAnthropic(Config{APIKey: "sk-ant-test", BaseURL: srv.URL})
	resp, err := client.Complete(context.Background(), Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 2048})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "first second" {
		t.Errorf("Text = %q, want every text block joined", resp.Text)
	}
}

// TestAnthropicOutputLimitLogCarriesNoText: the Warn line for a cut-off is
// written whatever the sensitive-logging setting says, so it may carry usage
// but never the user's text or the partial answer (#41).
func TestAnthropicOutputLimitLogCarriesNoText(t *testing.T) {
	var logs bytes.Buffer
	logger.InitWithWriter(&logs, "warning", false)
	t.Cleanup(func() { logger.InitWithWriter(io.Discard, "off", false) })

	const userText = "USER-SECRET-7731"
	const partial = "PARTIAL-ANSWER-4410"
	var got capture
	srv := newServer(t, &got, http.StatusOK,
		`{"stop_reason":"max_tokens","usage":{"input_tokens":10,"output_tokens":16000},"content":[{"type":"thinking","thinking":"","signature":"s"},{"type":"text","text":"`+partial+`"}]}`)

	client := newAnthropic(Config{APIKey: "sk-ant-test", BaseURL: srv.URL, Feature: "enhance"})
	_, err := client.Complete(context.Background(), Request{Model: "claude-sonnet-5", User: userText, MaxTokens: 16000})
	if err == nil {
		t.Fatal("expected an output-limit error")
	}

	line := logs.String()
	for _, want := range []string{"output limit reached", "output_tokens=16000", "thinking=true", "max_tokens=16000", "feature=enhance"} {
		if !strings.Contains(line, want) {
			t.Errorf("log = %q, want it to contain %q", line, want)
		}
	}
	for _, secret := range []string{userText, partial} {
		if strings.Contains(line, secret) {
			t.Errorf("log leaked %q: %q", secret, line)
		}
	}
}
