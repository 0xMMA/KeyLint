package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
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
