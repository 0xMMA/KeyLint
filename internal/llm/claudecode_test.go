package llm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// stubPath is the compiled stand-in for the Claude Code CLI. It is a real
// binary rather than a shell script so these tests also run on Windows, which
// is the platform KeyLint ships to and the one whose spawn path differs most.
var stubPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "keylint-claudestub")
	if err != nil {
		fmt.Fprintf(os.Stderr, "stub CLI: %v\n", err)
		os.Exit(1)
	}
	stubPath = filepath.Join(dir, claudeBinaryName())
	build := exec.Command("go", "build", "-o", stubPath, "./testdata/claudestub")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "stub CLI build failed: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// claudeBinaryName is what the CLI is called on this platform.
func claudeBinaryName() string {
	if runtime.GOOS == "windows" {
		return "claude.exe"
	}
	return "claude"
}

// stub drives the stand-in CLI and records what it was handed.
type stub struct {
	t         *testing.T
	argvFile  string
	stdinFile string
	envFile   string
}

// newStub points the stand-in at fresh recording files for one test.
func newStub(t *testing.T) *stub {
	t.Helper()
	dir := t.TempDir()
	s := &stub{
		t:         t,
		argvFile:  filepath.Join(dir, "argv"),
		stdinFile: filepath.Join(dir, "stdin"),
		envFile:   filepath.Join(dir, "env"),
	}
	t.Setenv("CLAUDESTUB_ARGV_FILE", s.argvFile)
	t.Setenv("CLAUDESTUB_STDIN_FILE", s.stdinFile)
	t.Setenv("CLAUDESTUB_ENV_FILE", s.envFile)
	return s
}

// replies makes the stand-in print out on stdout and exit successfully.
func (s *stub) replies(out string) { s.t.Setenv("CLAUDESTUB_STDOUT", out) }

// fails makes the stand-in print to stderr and exit with code.
func (s *stub) fails(stderr, code string) {
	s.t.Setenv("CLAUDESTUB_STDERR", stderr)
	s.t.Setenv("CLAUDESTUB_EXIT", code)
}

// hangs makes the stand-in spawn a child and then sleep. The child writes the
// returned marker path after a second unless the whole tree is killed.
func (s *stub) hangs() string {
	marker := filepath.Join(s.t.TempDir(), "survived")
	s.t.Setenv("CLAUDESTUB_HANG", "1")
	s.t.Setenv("CLAUDESTUB_MARKER", marker)
	return marker
}

func (s *stub) argv() []string { return splitRecorded(s.t, s.argvFile) }
func (s *stub) env() []string  { return splitRecorded(s.t, s.envFile) }
func (s *stub) stdin() string  { return readRecorded(s.t, s.stdinFile) }
func (s *stub) client() Client { return newClaudeCode(Config{CLIPath: stubPath}) }

func splitRecorded(t *testing.T, path string) []string {
	t.Helper()
	raw := readRecorded(t, path)
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}

func readRecorded(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the stub CLI recorded nothing at %s: %v", filepath.Base(path), err)
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

func containsArg(argv []string, want string) bool {
	for _, a := range argv {
		if a == want {
			return true
		}
	}
	return false
}

const successEnvelope = `{"result":"They are going to the meeting.","is_error":false,"duration_ms":2210,"total_cost_usd":0.0024}`

func TestClaudeCodeCompleteSuccess(t *testing.T) {
	s := newStub(t)
	s.replies(successEnvelope)

	resp, err := s.client().Complete(context.Background(), Request{
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
	if got := s.stdin(); got != "their going to the meting" {
		t.Errorf("stdin = %q, want the user message", got)
	}

	argv := s.argv()
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
	s := newStub(t)
	s.replies(successEnvelope)

	if _, err := s.client().Complete(context.Background(), Request{Model: "haiku", User: "x"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if containsArg(s.argv(), "--bare") {
		t.Error("--bare must never be passed: it skips credential reads")
	}
}

// TestClaudeCodeSystemPromptGoesThroughAFile pins the fix for Windows: the real
// system prompts run to several kilobytes with dozens of quotes and newlines,
// and on the usual claude.cmd shim cmd.exe re-parses the command line with
// rules Go does not escape for, under an 8191 character limit.
func TestClaudeCodeSystemPromptGoesThroughAFile(t *testing.T) {
	s := newStub(t)
	s.replies(successEnvelope)

	systemPrompt := "You are an assistant.\n\n**Rules:**\n1. Say \"hello\" and \"goodbye\"\n2. Keep 100% of the meaning\n" +
		strings.Repeat("Filler line with \"quotes\" and a % sign.\n", 200)

	if _, err := s.client().Complete(context.Background(), Request{
		System: systemPrompt, User: "x", Model: "haiku",
	}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	argv := s.argv()
	if containsArg(argv, "--system-prompt") {
		t.Error("--system-prompt puts kilobytes of quoted text on the command line; use --system-prompt-file")
	}
	path, ok := argValue(argv, "--system-prompt-file")
	if !ok {
		t.Fatalf("argv has no --system-prompt-file: %v", argv)
	}
	for _, arg := range argv {
		if strings.Contains(arg, "Filler line") {
			t.Fatal("the system prompt reached the command line")
		}
	}

	// The stub is still running when it records argv, so the file must exist then.
	// Afterwards it is cleaned up — check that too.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the staged system prompt file was left behind at %s", path)
	}
}

// TestClaudeCodeDropsInheritedAnthropicCredentials covers the promise the UI
// makes. KeyLint reads ANTHROPIC_API_KEY for its own BYOK providers, and the
// CLI treats that key as taking precedence over the user's claude.ai login — so
// inheriting it would bill a different account than "uses your own
// subscription" says, and an expired key makes the CLI hang with no output.
func TestClaudeCodeDropsInheritedAnthropicCredentials(t *testing.T) {
	s := newStub(t)
	s.replies(successEnvelope)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-should-not-be-inherited")
	t.Setenv("ANTHROPIC_BASE_URL", "https://proxy.invalid")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")

	if _, err := s.client().Complete(context.Background(), Request{Model: "haiku", User: "x"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var sawPath bool
	for _, entry := range s.env() {
		name, _, _ := strings.Cut(entry, "=")
		switch {
		case strings.EqualFold(name, "ANTHROPIC_API_KEY"),
			strings.EqualFold(name, "ANTHROPIC_BASE_URL"),
			strings.EqualFold(name, "CLAUDE_CODE_USE_BEDROCK"):
			t.Errorf("%s reached the CLI; it would override the user's own login", name)
		case strings.EqualFold(name, "PATH"):
			sawPath = true
		}
	}
	// Stripping must be surgical: the CLI still needs an ordinary environment.
	if !sawPath {
		t.Error("PATH did not reach the CLI")
	}
}

func TestClaudeCodeNotSignedIn(t *testing.T) {
	s := newStub(t)
	s.replies(`{"result":"Not logged in · Please run /login","is_error":true,"terminal_reason":"api_error"}`)
	s.fails("", "1")

	_, err := s.client().Complete(context.Background(), Request{Model: "haiku", User: "x"})
	if err == nil {
		t.Fatal("expected an error for a logged-out CLI")
	}
	if !strings.Contains(err.Error(), "run `claude`, and sign in") {
		t.Errorf("error = %v, want the actionable sign-in message", err)
	}
}

func TestClaudeCodeErrorEnvelope(t *testing.T) {
	s := newStub(t)
	s.replies(`{"result":"model \"nope\" is not available","is_error":true}`)

	_, err := s.client().Complete(context.Background(), Request{Model: "nope", User: "x"})
	if err == nil || !strings.Contains(err.Error(), `model "nope" is not available`) {
		t.Fatalf("error = %v, want the envelope result", err)
	}
}

func TestClaudeCodeExitFailure(t *testing.T) {
	s := newStub(t)
	// npm shims like to print an update notice first, so the last line is the
	// one that actually says what went wrong.
	s.fails("npm notice: a new version is available\nclaude: command failed", "3")

	_, err := s.client().Complete(context.Background(), Request{Model: "haiku", User: "x"})
	if err == nil || !strings.Contains(err.Error(), "claude: command failed") {
		t.Fatalf("error = %v, want the last stderr line", err)
	}
}

func TestClaudeCodeInvalidJSON(t *testing.T) {
	s := newStub(t)
	s.replies("not json at all")

	_, err := s.client().Complete(context.Background(), Request{Model: "haiku", User: "x"})
	if err == nil || !strings.Contains(err.Error(), "unexpected response") {
		t.Fatalf("error = %v, want an unexpected-response error", err)
	}
}

// TestClaudeCodeToleratesNoiseBeforeJSON keeps an update notice from breaking
// every call: the CLI and the npm shim both print to stdout on occasion.
func TestClaudeCodeToleratesNoiseBeforeJSON(t *testing.T) {
	s := newStub(t)
	s.replies("npm notice: a new version of claude is available\n" + successEnvelope)

	resp, err := s.client().Complete(context.Background(), Request{Model: "haiku", User: "x"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "They are going to the meeting." {
		t.Errorf("Text = %q, want the envelope result", resp.Text)
	}
}

// TestClaudeCodeEmptyResult guards the silent-fix hotkey: an empty result would
// be written to the clipboard and pasted over whatever the user had selected.
func TestClaudeCodeEmptyResult(t *testing.T) {
	s := newStub(t)
	s.replies(`{"result":"","is_error":false,"duration_ms":10}`)

	_, err := s.client().Complete(context.Background(), Request{Model: "haiku", User: "x"})
	if err == nil || !strings.Contains(err.Error(), "empty result") {
		t.Fatalf("error = %v, want an empty-result error", err)
	}
}

func TestClaudeCodeRequiresModel(t *testing.T) {
	client := newClaudeCode(Config{CLIPath: stubPath})
	_, err := client.Complete(context.Background(), Request{User: "x"})
	if err == nil || !strings.Contains(err.Error(), "model is required") {
		t.Fatalf("error = %v, want a missing-model error", err)
	}
}

func TestClaudeCodeMissingBinary(t *testing.T) {
	client := newClaudeCode(Config{CLIPath: filepath.Join(t.TempDir(), claudeBinaryName())})
	_, err := client.Complete(context.Background(), Request{Model: "haiku", User: "x"})
	if err == nil {
		t.Fatal("expected an error when the binary does not exist")
	}
}

// TestClaudeCodeTimeoutKillsProcessTree covers what a hotkey user feels: the
// call must come back at the deadline, and the CLI's own children must not
// survive it. The stub spawns a child that leaves a marker behind if it lives —
// the real npm shim spawns node the same way.
func TestClaudeCodeTimeoutKillsProcessTree(t *testing.T) {
	s := newStub(t)
	marker := s.hangs()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := s.client().Complete(ctx, Request{Model: "haiku", User: "x"})
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

	// Give the child more than its sleep to prove it was killed with the tree
	// rather than outliving the call.
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Error("a child of the CLI survived cancellation — the process tree was not killed")
	}
}

// TestClaudeCodeTimeoutKeepsStderr matters because the CLI's explanation for
// going quiet — an inherited credential, a proxy it cannot reach — arrives on
// stderr and nowhere else.
func TestClaudeCodeTimeoutKeepsStderr(t *testing.T) {
	s := newStub(t)
	s.hangs()
	t.Setenv("CLAUDESTUB_STDERR", "⚠ claude.ai connectors are disabled because ANTHROPIC_API_KEY is set")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := s.client().Complete(ctx, Request{Model: "haiku", User: "x"})
	if err == nil {
		t.Fatal("expected an error when the deadline passes")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY is set") {
		t.Errorf("error = %v, want it to carry what the CLI said on stderr", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want it to still wrap context.DeadlineExceeded", err)
	}
}
