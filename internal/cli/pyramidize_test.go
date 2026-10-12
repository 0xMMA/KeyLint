package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"keylint/internal/features/pyramidize"
)

type mockPyramidizer struct {
	result pyramidize.PyramidizeResult
	err    error
}

func (m *mockPyramidizer) Pyramidize(req pyramidize.PyramidizeRequest) (pyramidize.PyramidizeResult, error) {
	return m.result, m.err
}

func TestRunPyramidizeTextOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mock := &mockPyramidizer{
		result: pyramidize.PyramidizeResult{
			FullDocument: "Subject Line\n\nStructured body.",
			DocumentType: "EMAIL",
		},
	}
	err := runPyramidizeWith([]string{"-type", "email", "raw input text"}, &stdout, &stderr, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := stdout.String(); got != "Subject Line\n\nStructured body.\n" {
		t.Fatalf("got %q, want plain text output", got)
	}
}

// TestRunPyramidizeJSONOutput pins the --json shape: what was written and in
// which form, nothing else. qualityScore went (ADR-002 decision 6): it was
// the Go zero value for every v2 email, a documented output that lied.
func TestRunPyramidizeJSONOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mock := &mockPyramidizer{
		result: pyramidize.PyramidizeResult{
			FullDocument: "Subject Line\n\nBody.",
			DocumentType: "EMAIL",
			Language:     "en",
		},
	}
	err := runPyramidizeWith([]string{"--json", "-type", "email", "raw input"}, &stdout, &stderr, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, stdout.String())
	}
	want := map[string]any{"documentType": "EMAIL", "language": "en", "fullDocument": "Subject Line\n\nBody."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("--json = %v, want exactly %v", got, want)
	}
}

func TestRunPyramidizeDefaultType(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var capturedReq pyramidize.PyramidizeRequest
	mock := &mockPyramidizer{
		result: pyramidize.PyramidizeResult{
			FullDocument: "output",
		},
	}
	capturing := &capturingPyramidizer{mock: mock, captured: &capturedReq}
	err := runPyramidizeWith([]string{"some text"}, &stdout, &stderr, capturing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The CLI has no source app to look a preset up for, so its default is
	// what "auto" would resolve to anyway — said plainly.
	if capturedReq.DocumentType != "email" {
		t.Fatalf("documentType = %q, want %q", capturedReq.DocumentType, "email")
	}
}

type capturingPyramidizer struct {
	mock     *mockPyramidizer
	captured *pyramidize.PyramidizeRequest
}

func (c *capturingPyramidizer) Pyramidize(req pyramidize.PyramidizeRequest) (pyramidize.PyramidizeResult, error) {
	*c.captured = req
	return c.mock.Pyramidize(req)
}

func TestRunPyramidizeProviderOverride(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var capturedReq pyramidize.PyramidizeRequest
	mock := &mockPyramidizer{
		result: pyramidize.PyramidizeResult{
			FullDocument: "output",
		},
	}
	capturing := &capturingPyramidizer{mock: mock, captured: &capturedReq}
	err := runPyramidizeWith([]string{"--provider", "openai", "--model", "gpt-4o", "text"}, &stdout, &stderr, capturing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.Provider != "openai" {
		t.Fatalf("provider = %q, want %q", capturedReq.Provider, "openai")
	}
	if capturedReq.Model != "gpt-4o" {
		t.Fatalf("model = %q, want %q", capturedReq.Model, "gpt-4o")
	}
}

// TestRunPyramidizeOffersEveryForm: the help names the five forms, chat
// included, and the variants that still exist.
func TestRunPyramidizeOffersEveryForm(t *testing.T) {
	var stdout, stderr bytes.Buffer
	_ = runPyramidizeWith([]string{"-h"}, &stdout, &stderr, &mockPyramidizer{})
	help := stderr.String()
	for _, want := range []string{"email|chat|memo|wiki|powerpoint", "2=v2 email"} {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks %q:\n%s", want, help)
		}
	}
}

// TestRunPyramidizePassesChatThrough: -type chat reaches the service as is.
func TestRunPyramidizePassesChatThrough(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var capturedReq pyramidize.PyramidizeRequest
	capturing := &capturingPyramidizer{mock: &mockPyramidizer{result: pyramidize.PyramidizeResult{FullDocument: "x"}}, captured: &capturedReq}
	if err := runPyramidizeWith([]string{"-type", "chat", "text"}, &stdout, &stderr, capturing); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.DocumentType != "chat" {
		t.Fatalf("documentType = %q, want chat", capturedReq.DocumentType)
	}
}

// TestRunPyramidizeInstructions: --instructions is the CLI's version of the
// choice page's one field, and reaches the request as it does from the app.
func TestRunPyramidizeInstructions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var capturedReq pyramidize.PyramidizeRequest
	capturing := &capturingPyramidizer{mock: &mockPyramidizer{result: pyramidize.PyramidizeResult{FullDocument: "out"}}, captured: &capturedReq}
	if err := runPyramidizeWith([]string{"--instructions", "in English", "text"}, &stdout, &stderr, capturing); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.CustomInstructions != "in English" {
		t.Errorf("CustomInstructions = %q, want %q", capturedReq.CustomInstructions, "in English")
	}
}

// TestRunPyramidizeStillAcceptsStyleFlags: --style and --relationship do
// nothing since the choice page (no prompt carries them), but a script that
// passes them must keep running rather than fail on an unknown flag.
func TestRunPyramidizeStillAcceptsStyleFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var capturedReq pyramidize.PyramidizeRequest
	capturing := &capturingPyramidizer{mock: &mockPyramidizer{result: pyramidize.PyramidizeResult{FullDocument: "out"}}, captured: &capturedReq}
	err := runPyramidizeWith([]string{"--style", "casual", "--relationship", "close", "text"}, &stdout, &stderr, capturing)
	if err != nil {
		t.Fatalf("old flags were refused: %v", err)
	}
	if capturedReq.Text != "text" {
		t.Errorf("Text = %q, want %q", capturedReq.Text, "text")
	}
}
