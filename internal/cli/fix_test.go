package cli

import (
	"bytes"
	"testing"
)

type mockEnhancer struct {
	result string
	err    error
}

func (m *mockEnhancer) Enhance(text string) (string, error) {
	return m.result, m.err
}

func TestRunFixInlineString(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	enhancer := &mockEnhancer{result: "They're going to the meeting."}
	err := runFixWith([]string{"their going to the meeting"}, &stdout, &stderr, enhancer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := stdout.String(); got != "They're going to the meeting.\n" {
		t.Fatalf("got %q, want %q", got, "They're going to the meeting.\n")
	}
}

func TestRunFixNoInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	enhancer := &mockEnhancer{result: "fixed"}
	err := runFixWith(nil, &stdout, &stderr, enhancer)
	if err == nil {
		t.Fatal("expected error when no input")
	}
}
