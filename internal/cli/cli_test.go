package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// #46: stdin used to be consulted before the inline argument, so
// `KeyLint -fix "some text"` run with anything attached to stdin read that
// instead — and hung when nothing ever came.

func TestInlineTextBeatsStdin(t *testing.T) {
	got, err := readInput("", "from the command line", strings.NewReader("from stdin"))
	if err != nil {
		t.Fatalf("readInput: %v", err)
	}
	if got != "from the command line" {
		t.Errorf("got %q, want the argument the user typed", got)
	}
}

func TestAFileBeatsBoth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "in.txt")
	if err := os.WriteFile(path, []byte("from the file"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := readInput(path, "from the command line", strings.NewReader("from stdin"))
	if err != nil {
		t.Fatalf("readInput: %v", err)
	}
	if got != "from the file" {
		t.Errorf("got %q, want the file's contents", got)
	}
}

// TestASilentPipeFailsInsteadOfHanging: an open pipe that never delivers is the
// shape that made the command look like a slow AI call. os.Pipe gives a real
// one — nothing is ever written to the write end, and it is deliberately not
// closed, so the read genuinely blocks.
func TestASilentPipeFailsInsteadOfHanging(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })

	done := make(chan error, 1)
	start := time.Now()
	go func() {
		_, err := readInput("", "", r)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a silent pipe produced input")
		}
		if !strings.Contains(err.Error(), "stdin") {
			t.Errorf("error = %q, want it to name stdin so the user knows what to change", err)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("took %s — the deadline is meant to be about %s", elapsed, stdinFirstByteTimeout)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("readInput is still blocked on a silent pipe")
	}
}

// TestASlowPipeIsStillRead: the deadline is on the first byte, not on the whole
// read. A producer that takes its time must not be cut off.
func TestASlowPipeIsStillRead(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })

	go func() {
		_, _ = w.Write([]byte("first"))
		time.Sleep(stdinFirstByteTimeout + 500*time.Millisecond)
		_, _ = w.Write([]byte(" and the rest"))
		_ = w.Close()
	}()

	got, err := readInput("", "", r)
	if err != nil {
		t.Fatalf("readInput: %v", err)
	}
	if got != "first and the rest" {
		t.Errorf("got %q, want the whole slow message", got)
	}
}
