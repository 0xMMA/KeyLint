package llm

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestOpenAICompleteRequestShape(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"choices":[{"message":{"content":"fixed text"}}]}`)

	client := newOpenAI(Config{APIKey: "sk-test", BaseURL: srv.URL})
	resp, err := client.Complete(context.Background(), Request{
		System: "system prompt",
		User:   "user message",
		Model:  "gpt-4o-mini",
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
	if got.path != "/v1/chat/completions" {
		t.Errorf("path = %q, want /v1/chat/completions", got.path)
	}
	if h := got.headers.Get("Authorization"); h != "Bearer sk-test" {
		t.Errorf("Authorization = %q, want %q", h, "Bearer sk-test")
	}
	if h := got.headers.Get("Content-Type"); h != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", h)
	}
	if got.body["model"] != "gpt-4o-mini" {
		t.Errorf("model = %v, want gpt-4o-mini", got.body["model"])
	}
	if _, ok := got.body["response_format"]; ok {
		t.Error("response_format must be absent when the caller set no schema")
	}

	messages, ok := got.body["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("messages = %v, want two entries", got.body["messages"])
	}
	system := messages[0].(map[string]any)
	if system["role"] != "system" || system["content"] != "system prompt" {
		t.Errorf("first message = %v, want the system prompt", system)
	}
	user := messages[1].(map[string]any)
	if user["role"] != "user" || user["content"] != "user message" {
		t.Errorf("second message = %v, want the user message", user)
	}
}

func TestOpenAICompleteJSONSchema(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"choices":[{"message":{"content":"{}"}}]}`)

	client := newOpenAI(Config{APIKey: "sk-test", BaseURL: srv.URL})
	req := Request{Model: "gpt-5.2", User: "x", JSONSchema: []byte(testSchema)}
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	format, ok := got.body["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format = %v, want an object", got.body["response_format"])
	}
	if format["type"] != "json_schema" {
		t.Errorf("response_format.type = %v, want json_schema", format["type"])
	}
	spec, ok := format["json_schema"].(map[string]any)
	if !ok {
		t.Fatalf("json_schema = %v, want an object", format["json_schema"])
	}
	if spec["name"] != schemaName {
		t.Errorf("json_schema.name = %v, want %q", spec["name"], schemaName)
	}
	if spec["strict"] != true {
		t.Errorf("json_schema.strict = %v, want true", spec["strict"])
	}
	schema, ok := spec["schema"].(map[string]any)
	if !ok {
		t.Fatalf("json_schema.schema = %v, want the caller's schema", spec["schema"])
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok || properties["answer"] == nil {
		t.Errorf("schema did not survive: %v", schema)
	}
}

func TestOpenAICompleteWithoutSchema(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"choices":[{"message":{"content":"plain"}}]}`)

	client := newOpenAI(Config{APIKey: "sk-test", BaseURL: srv.URL})
	if _, err := client.Complete(context.Background(), Request{Model: "gpt-4o-mini", User: "x"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// enhance sends no schema and must not be constrained.
	if _, ok := got.body["response_format"]; ok {
		t.Error("response_format must be absent when the caller set no schema")
	}
}

func TestOpenAICompleteErrorStatus(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusUnauthorized, `{"error":{"message":"invalid key","type":"invalid_request_error"}}`)

	client := newOpenAI(Config{APIKey: "bad", BaseURL: srv.URL})
	_, err := client.Complete(context.Background(), Request{Model: "gpt-4o-mini", User: "x"})
	if err == nil {
		t.Fatal("expected an error for status 401")
	}
	if !strings.Contains(err.Error(), "OpenAI error 401") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOpenAICompleteUnexpectedResponse(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"choices":[]}`)

	client := newOpenAI(Config{APIKey: "sk-test", BaseURL: srv.URL})
	_, err := client.Complete(context.Background(), Request{Model: "gpt-4o-mini", User: "x"})
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOpenAICompleteRequiresModel(t *testing.T) {
	client := newOpenAI(Config{})
	_, err := client.Complete(context.Background(), Request{User: "x"})
	if err == nil || !strings.Contains(err.Error(), "model is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestOpenAIIgnoresUnsupportedFields pins the wire format against #33 step 3:
// when the vendor SDK lands it must not start sending max_tokens for a Request
// that carries one today, because the hand-rolled client never did.
func TestOpenAIIgnoresUnsupportedFields(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"choices":[{"message":{"content":"ok"}}]}`)

	client := newOpenAI(Config{APIKey: "sk-test", BaseURL: srv.URL})
	if _, err := client.Complete(context.Background(), Request{Model: "gpt-4o-mini", User: "x", MaxTokens: 2048}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if _, ok := got.body["max_tokens"]; ok {
		t.Error("max_tokens must not reach the OpenAI payload")
	}
}

// TestOpenAIJSONModeWithoutSchema covers the weaker constraint: Pyramidize
// parses JSON on every step, so with schema enforcement off it still has to ask
// for an object — which is what it did before schemas existed.
func TestOpenAIJSONModeWithoutSchema(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"choices":[{"message":{"content":"{}"}}]}`)

	client := newOpenAI(Config{APIKey: "sk-test", BaseURL: srv.URL})
	if _, err := client.Complete(context.Background(), Request{Model: "gpt-5.2", User: "x", JSONMode: true}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	format, ok := got.body["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format = %v, want an object", got.body["response_format"])
	}
	if format["type"] != "json_object" {
		t.Errorf("response_format.type = %v, want json_object", format["type"])
	}
}

// TestOpenAISchemaWinsOverJSONMode: both set is what Pyramidize sends while
// enforcement is on, and the shape has to win.
func TestOpenAISchemaWinsOverJSONMode(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"choices":[{"message":{"content":"{}"}}]}`)

	client := newOpenAI(Config{APIKey: "sk-test", BaseURL: srv.URL})
	req := Request{Model: "gpt-5.2", User: "x", JSONMode: true, JSONSchema: []byte(testSchema)}
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	format := got.body["response_format"].(map[string]any)
	if format["type"] != "json_schema" {
		t.Errorf("response_format.type = %v, want json_schema to win", format["type"])
	}
}

// TestOpenAIRefusesUnusableAnswers covers the three outcomes strict schemas made
// reachable. Each would otherwise be handed to the caller as text — and for the
// fix flow, written straight over the user's selection.
func TestOpenAIRefusesUnusableAnswers(t *testing.T) {
	tests := []struct {
		name  string
		reply string
		want  string
	}{
		{
			name:  "cut off at the output limit",
			reply: `{"choices":[{"finish_reason":"length","message":{"content":"They are going to the"}}]}`,
			want:  "exceeded the output limit",
		},
		{
			name:  "model declined",
			reply: `{"choices":[{"finish_reason":"stop","message":{"content":"","refusal":"I cannot help with that"}}]}`,
			want:  "declined the request",
		},
		{
			name:  "empty content",
			reply: `{"choices":[{"finish_reason":"stop","message":{"content":""}}]}`,
			want:  "empty result",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got capture
			srv := newServer(t, &got, http.StatusOK, tc.reply)

			client := newOpenAI(Config{APIKey: "sk-test", BaseURL: srv.URL})
			_, err := client.Complete(context.Background(), Request{Model: "gpt-4o-mini", User: "x"})
			if err == nil {
				t.Fatal("expected an error rather than an unusable answer")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}
