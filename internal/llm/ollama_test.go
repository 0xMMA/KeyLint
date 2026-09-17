package llm

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// Ollama now goes through its OpenAI-compatible endpoint rather than the native
// /api/generate, so these tests pin the OpenAI wire format against an Ollama
// base URL — including the thing the change bought us: a real system message
// instead of the system prompt glued onto the front of the user text.

func TestOllamaCompleteRequestShape(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"choices":[{"message":{"content":"fixed text"}}]}`)

	client := newOllama(Config{BaseURL: srv.URL})
	resp, err := client.Complete(context.Background(), Request{
		System: "system prompt",
		User:   "user message",
		Model:  "llama3.2",
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
	if got.body["model"] != "llama3.2" {
		t.Errorf("model = %v, want llama3.2", got.body["model"])
	}

	messages, ok := got.body["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("messages = %v, want two entries", got.body["messages"])
	}
	system := messages[0].(map[string]any)
	if system["role"] != "system" || system["content"] != "system prompt" {
		t.Errorf("first message = %v, want the system prompt in a system role", system)
	}
	user := messages[1].(map[string]any)
	if user["role"] != "user" || user["content"] != "user message" {
		t.Errorf("second message = %v, want the user message", user)
	}

	// The native endpoint's fields must not reappear.
	if _, ok := got.body["prompt"]; ok {
		t.Error("prompt must not be sent to the chat completions endpoint")
	}
	if _, ok := got.body["stream"]; ok {
		t.Error("stream must not be sent: these are one-shot calls")
	}
}

func TestOllamaCompleteJSONMode(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"choices":[{"message":{"content":"{}"}}]}`)

	client := newOllama(Config{BaseURL: srv.URL})
	if _, err := client.Complete(context.Background(), Request{Model: "llama3.2", User: "x", JSONMode: true}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// Ollama's OpenAI-compatible endpoint honours this, which the native one did not.
	format, ok := got.body["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format = %v, want an object", got.body["response_format"])
	}
	if format["type"] != "json_object" {
		t.Errorf("response_format.type = %v, want json_object", format["type"])
	}
}

func TestOllamaCompleteErrorStatus(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusNotFound, `{"error":{"message":"model not found"}}`)

	client := newOllama(Config{BaseURL: srv.URL})
	_, err := client.Complete(context.Background(), Request{Model: "llama3.2", User: "x"})
	if err == nil {
		t.Fatal("expected an error for status 404")
	}
	if !strings.Contains(err.Error(), "Ollama error 404") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOllamaCompleteNoChoices(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"choices":[]}`)

	client := newOllama(Config{BaseURL: srv.URL})
	_, err := client.Complete(context.Background(), Request{Model: "llama3.2", User: "x"})
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOllamaCompleteRequiresModel(t *testing.T) {
	client := newOllama(Config{})
	_, err := client.Complete(context.Background(), Request{User: "x"})
	if err == nil || !strings.Contains(err.Error(), "model is required") {
		t.Errorf("unexpected error: %v", err)
	}
}
