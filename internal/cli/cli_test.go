package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"keylint/internal/features/pyramidize"
)

func TestRunUnknownCommand(t *testing.T) {
	var stderr bytes.Buffer
	err := Run([]string{"-unknown"}, nil, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	want := `unknown command: "-unknown"`
	if err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}
}

func TestRunNoArgs(t *testing.T) {
	err := Run(nil, nil, nil)
	if err == nil {
		t.Fatal("expected error when no args provided")
	}
}

func TestReadInputFromString(t *testing.T) {
	got, err := readInput("", "hello world", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello world" {
		t.Fatalf("got %q, want %q", got, "hello world")
	}
}

func TestReadInputFromFile(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/input.txt"
	if err := os.WriteFile(path, []byte("file content"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := readInput(path, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "file content" {
		t.Fatalf("got %q, want %q", got, "file content")
	}
}

func TestReadInputFromStdin(t *testing.T) {
	stdin := strings.NewReader("stdin content")
	got, err := readInput("", "", stdin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "stdin content" {
		t.Fatalf("got %q, want %q", got, "stdin content")
	}
}

func TestReadInputNone(t *testing.T) {
	_, err := readInput("", "", nil)
	if err == nil {
		t.Fatal("expected error when no input provided")
	}
}

func TestReadInputFilePriority(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/input.txt"
	if err := os.WriteFile(path, []byte("from file"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := readInput(path, "from string", strings.NewReader("from stdin"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from file" {
		t.Fatalf("got %q, want %q", got, "from file")
	}
}

func TestLogFlagValidation_InvalidLevel(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mock := &mockEnhancer{result: "fixed"}
	err := runFixWith([]string{"--log", "banana", "hello"}, &stdout, &stderr, mock)
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
	if !strings.Contains(err.Error(), "invalid log level") {
		t.Fatalf("error %q should contain %q", err.Error(), "invalid log level")
	}
}

func TestLogFlagValidation_ValidLevels(t *testing.T) {
	levels := []string{"off", "trace", "debug", "info", "warning", "error"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			mock := &mockEnhancer{result: "fixed"}
			err := runFixWith([]string{"--log", level, "hello"}, &stdout, &stderr, mock)
			if err != nil {
				t.Fatalf("unexpected error for level %q: %v", level, err)
			}
		})
	}
}

func TestLogFlagDefault_IsOff(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mock := &mockEnhancer{result: "fixed"}
	err := runFixWith([]string{"hello"}, &stdout, &stderr, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPyramidizeLogFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mock := &mockPyramidizer{
		result: pyramidize.PyramidizeResult{
			FullDocument: "output",
			QualityFlags: []string{},
		},
	}
	err := runPyramidizeWith([]string{"--log", "debug", "hello"}, &stdout, &stderr, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
