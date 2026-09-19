package llm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// isolateCLILookup points discovery at empty directories and clears the
// machine-wide candidates, so a real Claude Code installation on the
// developer's machine cannot decide a test's outcome — and so no test is
// silently skipped on a machine that happens to have one.
func isolateCLILookup(t *testing.T) string {
	t.Helper()
	original := systemCandidates
	systemCandidates = nil
	t.Cleanup(func() { systemCandidates = original })

	dir := t.TempDir()
	t.Setenv("PATH", dir)
	// os.UserHomeDir reads HOME on Unix and USERPROFILE on Windows.
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

// installStubAt puts a runnable stand-in CLI at path.
func installStubAt(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	binary, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("read stub CLI: %v", err)
	}
	if err := os.WriteFile(path, binary, 0o755); err != nil {
		t.Fatalf("write stub CLI: %v", err)
	}
}

func TestLocateClaudeCodeOnPath(t *testing.T) {
	dir := isolateCLILookup(t)
	binary := filepath.Join(dir, claudeBinaryName())
	installStubAt(t, binary)

	got, err := LocateClaudeCode()
	if err != nil {
		t.Fatalf("LocateClaudeCode: %v", err)
	}
	if got != binary {
		t.Errorf("LocateClaudeCode = %q, want %q", got, binary)
	}
}

// TestLocateClaudeCodeOffPath covers the case the roadmap calls out: an app
// started from a desktop launcher often has a shorter PATH than a terminal, so
// the known install directories have to be probed too.
func TestLocateClaudeCodeOffPath(t *testing.T) {
	home := isolateCLILookup(t)
	binary := filepath.Join(home, ".local", "bin", claudeBinaryName())
	installStubAt(t, binary)

	got, err := LocateClaudeCode()
	if err != nil {
		t.Fatalf("LocateClaudeCode: %v", err)
	}
	if got != binary {
		t.Errorf("LocateClaudeCode = %q, want the binary in ~/.local/bin", got)
	}
}

func TestLocateClaudeCodeNotInstalled(t *testing.T) {
	isolateCLILookup(t)

	if _, err := LocateClaudeCode(); err == nil {
		t.Fatal("expected an error when no binary is installed")
	}
}

// TestLocateClaudeCodeIgnoresNonExecutable guards against reporting a stray file
// named "claude" as an installation. Windows decides by extension instead, so
// there is nothing to check there.
func TestLocateClaudeCodeIgnoresNonExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows treats any file with the right extension as runnable")
	}
	home := isolateCLILookup(t)
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "claude"), []byte("notes"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := LocateClaudeCode(); err == nil {
		t.Fatal("a non-executable file must not count as an installation")
	}
}

func TestCheckClaudeCodeSignedIn(t *testing.T) {
	t.Setenv("CLAUDESTUB_VERSION", "2.1.274 (Claude Code)")
	t.Setenv("CLAUDESTUB_AUTH", `{"loggedIn":true,"authMethod":"claude.ai","email":"someone@example.com"}`)

	status := CheckClaudeCode(context.Background(), stubPath)
	if !status.Installed {
		t.Error("Installed = false, want true")
	}
	if status.Path != stubPath {
		t.Errorf("Path = %q, want %q", status.Path, stubPath)
	}
	if status.Version != "2.1.274" {
		t.Errorf("Version = %q, want 2.1.274", status.Version)
	}
	if !status.LoggedIn {
		t.Error("LoggedIn = false, want true")
	}
}

func TestCheckClaudeCodeSignedOut(t *testing.T) {
	t.Setenv("CLAUDESTUB_VERSION", "2.1.274 (Claude Code)")
	t.Setenv("CLAUDESTUB_AUTH", `{"loggedIn":false}`)

	status := CheckClaudeCode(context.Background(), stubPath)
	if !status.Installed {
		t.Error("Installed = false, want true — the binary is there, only the session is not")
	}
	if status.LoggedIn {
		t.Error("LoggedIn = true, want false")
	}
}

// TestCheckClaudeCodeProbesWithoutInheritedCredentials pins the fix for a bug
// where detection and execution disagreed: the probe inherited KeyLint's own
// ANTHROPIC_API_KEY, so `claude auth status` reported an API-key session as
// signed in, the UI offered the one-click path, and every completion — which
// runs without that key — then failed as "not signed in".
func TestCheckClaudeCodeProbesWithoutInheritedCredentials(t *testing.T) {
	s := newStub(t)
	t.Setenv("CLAUDESTUB_VERSION", "2.1.274 (Claude Code)")
	t.Setenv("CLAUDESTUB_AUTH", `{"loggedIn":true,"authMethod":"api_key"}`)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-should-not-be-inherited")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "token-should-not-be-inherited")

	CheckClaudeCode(context.Background(), stubPath)

	var sawPath bool
	for _, entry := range s.env() {
		name, _, _ := strings.Cut(entry, "=")
		switch {
		case strings.EqualFold(name, "ANTHROPIC_API_KEY"),
			strings.EqualFold(name, "CLAUDE_CODE_OAUTH_TOKEN"):
			t.Errorf("%s reached the probe; the CLI would report a session the completion call cannot use", name)
		case strings.EqualFold(name, "PATH"):
			sawPath = true
		}
	}
	// Stripping must be surgical here too: the probe still needs to run.
	if !sawPath {
		t.Error("PATH did not reach the probe")
	}
}

func TestCheckClaudeCodeNotInstalled(t *testing.T) {
	isolateCLILookup(t)

	status := CheckClaudeCode(context.Background(), "")
	if status.Installed || status.LoggedIn || status.Path != "" {
		t.Errorf("status = %+v, want the zero value", status)
	}
}

// TestClaudeCodeStatusCarriesNoAccountDetails pins a privacy promise: `claude
// auth status` also prints the account's email, organisation and subscription,
// and none of that may travel to the frontend. Marshalling the whole struct
// means the test also fails if somebody adds such a field later.
func TestClaudeCodeStatusCarriesNoAccountDetails(t *testing.T) {
	t.Setenv("CLAUDESTUB_VERSION", "2.1.274 (Claude Code)")
	t.Setenv("CLAUDESTUB_AUTH", `{"loggedIn":true,"authMethod":"claude.ai","email":"someone@example.com",`+
		`"orgId":"f55e112d","orgName":"Someone's Org","subscriptionType":"max"}`)

	status := CheckClaudeCode(context.Background(), stubPath)
	rendered, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	for _, secret := range []string{"someone@example.com", "f55e112d", "Someone's Org", "max", "authMethod"} {
		if strings.Contains(string(rendered), secret) {
			t.Errorf("status leaked %q: %s", secret, rendered)
		}
	}
}
