package llm

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestOllamaCompleteRequestShape(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"response":"fixed text"}`)

	client := newOllama(Config{BaseURL: srv.URL, PromptSeparator: "\n\n---\n\n"})
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
	if got.path != "/api/generate" {
		t.Errorf("path = %q, want /api/generate", got.path)
	}
	if h := got.headers.Get("Content-Type"); h != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", h)
	}
	if got.body["model"] != "llama3.2" {
		t.Errorf("model = %v, want llama3.2", got.body["model"])
	}
	if got.body["stream"] != false {
		t.Errorf("stream = %v, want false", got.body["stream"])
	}
	if want := "system prompt\n\n---\n\nuser message"; got.body["prompt"] != want {
		t.Errorf("prompt = %q, want %q", got.body["prompt"], want)
	}
	if got.headers.Get("Authorization") != "" {
		t.Error("Ollama must not send an Authorization header")
	}
}

func TestOllamaCompleteDefaultSeparator(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"response":"ok"}`)

	client := newOllama(Config{BaseURL: srv.URL})
	if _, err := client.Complete(context.Background(), Request{System: "a", User: "b", Model: "llama3.2"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if want := "a\n\nb"; got.body["prompt"] != want {
		t.Errorf("prompt = %q, want %q", got.body["prompt"], want)
	}
}

func TestOllamaCompleteErrorStatus(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusNotFound, `model not found`)

	client := newOllama(Config{BaseURL: srv.URL})
	_, err := client.Complete(context.Background(), Request{Model: "llama3.2", User: "x"})
	if err == nil {
		t.Fatal("expected an error for status 404")
	}
	if !strings.Contains(err.Error(), "Ollama error 404") || !strings.Contains(err.Error(), "model not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOllamaCompleteUnexpectedResponse(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `not json`)

	client := newOllama(Config{BaseURL: srv.URL})
	_, err := client.Complete(context.Background(), Request{Model: "llama3.2", User: "x"})
	if err == nil || !strings.Contains(err.Error(), "Ollama unexpected response") {
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

// TestOllamaIgnoresUnsupportedFields pins the wire format against #33 step 3:
// Ollama honours neither field today and must not start to.
func TestOllamaIgnoresUnsupportedFields(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK, `{"response":"ok"}`)

	client := newOllama(Config{BaseURL: srv.URL})
	req := Request{Model: "llama3.2", User: "x", MaxTokens: 4096, JSONMode: true}
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if _, ok := got.body["max_tokens"]; ok {
		t.Error("max_tokens must not reach the Ollama payload")
	}
	if _, ok := got.body["response_format"]; ok {
		t.Error("response_format must not reach the Ollama payload")
	}
}
