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
	shortenStdinIdleTimeout(t)
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
		if elapsed := time.Since(start); elapsed > stdinIdleTimeout+5*time.Second {
			t.Errorf("took %s — the deadline is meant to be about %s", elapsed, stdinIdleTimeout)
		}
	case <-time.After(stdinIdleTimeout + 10*time.Second):
		t.Fatal("readInput is still blocked on a silent pipe")
	}
}

// TestASlowProducerIsStillRead: the clock is reset by every chunk, so a
// producer that dawdles between writes must not be cut off. `pdftotext big.pdf
// - | keylint -fix` is the shape this protects.
func TestASlowProducerIsStillRead(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })

	go func() {
		for _, part := range []string{"first", " and", " the rest"} {
			time.Sleep(150 * time.Millisecond)
			_, _ = w.Write([]byte(part))
		}
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

// TestAPipeThatSpeaksOnceAndStaysOpenGivesUp is the other half of #46, and the
// half a first-byte deadline misses entirely: a producer that emits a header
// line and then holds the descriptor open. `{ echo; sleep 3600; } | keylint
// -fix` used to hang forever.
func TestAPipeThatSpeaksOnceAndStaysOpenGivesUp(t *testing.T) {
	shortenStdinIdleTimeout(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })

	if _, err := w.Write([]byte("a header line\n")); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := readInput("", "", r)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a pipe that never closes produced a complete input")
		}
		if !strings.Contains(err.Error(), "quiet") {
			t.Errorf("error = %q, want it to say the producer went quiet", err)
		}
	case <-time.After(stdinIdleTimeout + 10*time.Second):
		t.Fatal("readInput is still blocked on a pipe that stopped writing")
	}
}

// shortenStdinIdleTimeout makes the deadline tests cheap. They are about the
// shape — that a quiet pipe ends in an error rather than a hang — and waiting
// out the real fifteen seconds twice would add half a minute to every CI run.
func shortenStdinIdleTimeout(t *testing.T) {
	t.Helper()
	original := stdinIdleTimeout
	stdinIdleTimeout = 200 * time.Millisecond
	t.Cleanup(func() { stdinIdleTimeout = original })
}

// TestALoneHyphenMeansStdin: the conventional spelling. Treating it as literal
// text sent a single hyphen to the model and ignored the pipe.
func TestALoneHyphenMeansStdin(t *testing.T) {
	got, err := readInput("", "-", strings.NewReader("piped text"))
	if err != nil {
		t.Fatalf("readInput: %v", err)
	}
	if got != "piped text" {
		t.Errorf("got %q, want what was piped in", got)
	}
}

// TestAHyphenInsideRealTextIsStillText: only a lone "-" is the convention.
func TestAHyphenInsideRealTextIsStillText(t *testing.T) {
	got, err := readInput("", "fix this - please", strings.NewReader("piped"))
	if err != nil {
		t.Fatalf("readInput: %v", err)
	}
	if got != "fix this - please" {
		t.Errorf("got %q, want the argument", got)
	}
}

// TestStdinIsCapped: `yes | KeyLint -fix` is one typo away, and the old reader
// would have grown a slice until the process died.
func TestStdinIsCapped(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })

	go func() {
		chunk := bytes.Repeat([]byte("x"), 1<<20)
		for i := 0; i < 64; i++ {
			if _, err := w.Write(chunk); err != nil {
				break
			}
		}
		_ = w.Close()
	}()

	_, err = readInput("", "", r)
	if err == nil {
		t.Fatal("an endless pipe was read to completion")
	}
	if !strings.Contains(err.Error(), "larger than") {
		t.Errorf("error = %q, want it to name the size limit", err)
	}
}
