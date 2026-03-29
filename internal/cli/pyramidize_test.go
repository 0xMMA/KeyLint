package cli

import (
	"bytes"
	"encoding/json"
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
			QualityScore: 0.85,
			QualityFlags: []string{},
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

func TestRunPyramidizeJSONOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mock := &mockPyramidizer{
		result: pyramidize.PyramidizeResult{
			FullDocument: "Subject Line\n\nBody.",
			DocumentType: "EMAIL",
			QualityScore: 0.85,
			QualityFlags: []string{},
		},
	}
	err := runPyramidizeWith([]string{"--json", "-type", "email", "raw input"}, &stdout, &stderr, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result pyramidize.PyramidizeResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, stdout.String())
	}
	if result.QualityScore != 0.85 {
		t.Fatalf("qualityScore = %v, want 0.85", result.QualityScore)
	}
}

func TestRunPyramidizeDefaultType(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var capturedReq pyramidize.PyramidizeRequest
	mock := &mockPyramidizer{
		result: pyramidize.PyramidizeResult{
			FullDocument: "output",
			QualityFlags: []string{},
		},
	}
	capturing := &capturingPyramidizer{mock: mock, captured: &capturedReq}
	err := runPyramidizeWith([]string{"some text"}, &stdout, &stderr, capturing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.DocumentType != "auto" {
		t.Fatalf("documentType = %q, want %q", capturedReq.DocumentType, "auto")
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
			QualityFlags: []string{},
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
