package llm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeCLI is a stub `claude` binary. It records the argv and stdin it was given
// so tests can assert the exact command line, then behaves as the script says.
type fakeCLI struct {
	path      string
	argvFile  string
	stdinFile string
}

// newFakeCLI writes an executable shell script standing in for the real CLI.
// body runs after argv and stdin have been recorded.
func newFakeCLI(t *testing.T, body string) *fakeCLI {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub CLI is a shell script; the Windows spawn path is covered by review, not by this test")
	}

	dir := t.TempDir()
	cli := &fakeCLI{
		path:      filepath.Join(dir, "claude"),
		argvFile:  filepath.Join(dir, "argv"),
		stdinFile: filepath.Join(dir, "stdin"),
	}
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + cli.argvFile + "\n" +
		"cat > " + cli.stdinFile + "\n" +
		body + "\n"
	if err := os.WriteFile(cli.path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub CLI: %v", err)
	}
	return cli
}

// argv returns the arguments the stub received, one per element.
func (f *fakeCLI) argv(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(f.argvFile)
	if err != nil {
		t.Fatalf("stub CLI recorded no argv: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

// stdin returns what was piped into the stub.
func (f *fakeCLI) stdin(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(f.stdinFile)
	if err != nil {
		t.Fatalf("stub CLI recorded no stdin: %v", err)
	}
	return string(raw)
}

// argValue returns the value following flag in argv.
func argValue(argv []string, flag string) (string, bool) {
	for i, a := range argv {
		if a == flag && i+1 < len(argv) {
			return argv[i+1], true
		}
	}
	return "", false
}

func TestClaudeCodeCompleteSuccess(t *testing.T) {
	cli := newFakeCLI(t, `echo '{"result":"They are going to the meeting.","is_error":false,"duration_ms":2210,"total_cost_usd":0.0024}'`)

	client := newClaudeCode(Config{CLIPath: cli.path, Feature: "enhance"})
	resp, err := client.Complete(context.Background(), Request{
		System: "fix the grammar",
		User:   "their going to the meting",
		Model:  "haiku",
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "They are going to the meeting." {
		t.Errorf("Text = %q, want the envelope result", resp.Text)
	}

	// The prompt goes in on stdin, so it never shows up in the process table.
	if got := cli.stdin(t); got != "their going to the meting" {
		t.Errorf("stdin = %q, want the user message", got)
	}

	argv := cli.argv(t)
	if argv[0] != "-p" {
		t.Errorf("argv[0] = %q, want -p (print mode)", argv[0])
	}
	for _, want := range []string{"--no-session-persistence", "--strict-mcp-config", "--disable-slash-commands"} {
		if !containsArg(argv, want) {
			t.Errorf("argv is missing %s: %v", want, argv)
		}
	}
	if v, ok := argValue(argv, "--model"); !ok || v != "haiku" {
		t.Errorf("--model = %q, want haiku", v)
	}
	if v, ok := argValue(argv, "--output-format"); !ok || v != "json" {
		t.Errorf("--output-format = %q, want json", v)
	}
	if v, ok := argValue(argv, "--system-prompt"); !ok || v != "fix the grammar" {
		t.Errorf("--system-prompt = %q, want the system prompt", v)
	}
	// --tools must be present with an empty value: the CLI must not be allowed
	// to touch files or run commands on a user's machine for a text fix.
	if v, ok := argValue(argv, "--tools"); !ok || v != "" {
		t.Errorf("--tools = %q (present=%v), want an empty value", v, ok)
	}
	if v, ok := argValue(argv, "--setting-sources"); !ok || v != "" {
		t.Errorf("--setting-sources = %q (present=%v), want an empty value", v, ok)
	}
}

// TestClaudeCodeNeverPassesBare guards the gotcha that cost the most time to
// find: --bare skips credential reads, so a signed-in user looks signed out.
func TestClaudeCodeNeverPassesBare(t *testing.T) {
	cli := newFakeCLI(t, `echo '{"result":"ok","is_error":false}'`)

	client := newClaudeCode(Config{CLIPath: cli.path})
	if _, err := client.Complete(context.Background(), Request{Model: "haiku", User: "x"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if containsArg(cli.argv(t), "--bare") {
		t.Error("--bare must never be passed: it skips credential reads")
	}
}

func TestClaudeCodeNotSignedIn(t *testing.T) {
	cli := newFakeCLI(t, `echo '{"result":"Not logged in · Please run /login","is_error":true,"terminal_reason":"error"}'`)

	client := newClaudeCode(Config{CLIPath: cli.path})
	_, err := client.Complete(context.Background(), Request{Model: "haiku", User: "x"})
	if err == nil {
		t.Fatal("expected an error for a logged-out CLI")
	}
	if !strings.Contains(err.Error(), "run `claude`, and sign in") {
		t.Errorf("error = %v, want the actionable sign-in message", err)
	}
}

func TestClaudeCodeErrorEnvelope(t *testing.T) {
	cli := newFakeCLI(t, `echo '{"result":"model \"nope\" is not available","is_error":true}'`)

	client := newClaudeCode(Config{CLIPath: cli.path})
	_, err := client.Complete(context.Background(), Request{Model: "nope", User: "x"})
	if err == nil || !strings.Contains(err.Error(), `model "nope" is not available`) {
		t.Fatalf("error = %v, want the envelope result", err)
	}
}

func TestClaudeCodeExitFailure(t *testing.T) {
	cli := newFakeCLI(t, "echo 'claude: command failed' >&2\nexit 3")

	client := newClaudeCode(Config{CLIPath: cli.path})
	_, err := client.Complete(context.Background(), Request{Model: "haiku", User: "x"})
	if err == nil || !strings.Contains(err.Error(), "claude: command failed") {
		t.Fatalf("error = %v, want the first stderr line", err)
	}
}

func TestClaudeCodeInvalidJSON(t *testing.T) {
	cli := newFakeCLI(t, `echo 'not json at all'`)

	client := newClaudeCode(Config{CLIPath: cli.path})
	_, err := client.Complete(context.Background(), Request{Model: "haiku", User: "x"})
	if err == nil || !strings.Contains(err.Error(), "unexpected response") {
		t.Fatalf("error = %v, want an unexpected-response error", err)
	}
}

func TestClaudeCodeRequiresModel(t *testing.T) {
	client := newClaudeCode(Config{CLIPath: "/nonexistent"})
	_, err := client.Complete(context.Background(), Request{User: "x"})
	if err == nil || !strings.Contains(err.Error(), "model is required") {
		t.Fatalf("error = %v, want a missing-model error", err)
	}
}

func TestClaudeCodeMissingBinary(t *testing.T) {
	client := newClaudeCode(Config{CLIPath: filepath.Join(t.TempDir(), "claude")})
	_, err := client.Complete(context.Background(), Request{Model: "haiku", User: "x"})
	if err == nil {
		t.Fatal("expected an error when the binary does not exist")
	}
}

// TestClaudeCodeTimeoutKillsProcessGroup covers what a hotkey user feels: the
// call must come back at the deadline, and the CLI's own children must not
// survive it. The stub forks a child that leaves a marker behind if it lives.
func TestClaudeCodeTimeoutKillsProcessGroup(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "survived")
	cli := newFakeCLI(t, "( sleep 1; echo alive > "+marker+" ) &\nsleep 30")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	client := newClaudeCode(Config{CLIPath: cli.path})
	_, err := client.Complete(ctx, Request{Model: "haiku", User: "x"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error when the deadline passes")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want one wrapping context.DeadlineExceeded", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("Complete took %v, want a return at the deadline", elapsed)
	}

	// Give the forked child more than its sleep to prove it was killed with the
	// group rather than outliving the call.
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Error("a child of the CLI survived cancellation — the process group was not killed")
	}
}

func containsArg(argv []string, want string) bool {
	for _, a := range argv {
		if a == want {
			return true
		}
	}
	return false
}
