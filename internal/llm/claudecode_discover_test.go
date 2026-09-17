package llm

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// isolateCLILookup points discovery at empty directories so a real Claude Code
// installation on the developer's machine cannot decide the outcome.
func isolateCLILookup(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub CLI is a shell script; the Windows lookup path is covered by review, not by this test")
	}
	for _, systemPath := range []string{"/usr/local/bin/claude", "/opt/homebrew/bin/claude"} {
		if isExecutable(systemPath) {
			t.Skipf("%s exists on this machine, so discovery cannot be isolated", systemPath)
		}
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("HOME", dir)
	return dir
}

func TestLocateClaudeCodeOnPath(t *testing.T) {
	dir := isolateCLILookup(t)
	binary := filepath.Join(dir, "claude")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write stub CLI: %v", err)
	}

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
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	binary := filepath.Join(binDir, "claude")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write stub CLI: %v", err)
	}

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
// named "claude" as an installation.
func TestLocateClaudeCodeIgnoresNonExecutable(t *testing.T) {
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
	cli := newStatusCLI(t, "2.1.274 (Claude Code)", `{"loggedIn":true,"authMethod":"claude.ai","email":"someone@example.com"}`)

	status := CheckClaudeCode(context.Background(), cli)
	if !status.Installed {
		t.Error("Installed = false, want true")
	}
	if status.Path != cli {
		t.Errorf("Path = %q, want %q", status.Path, cli)
	}
	if status.Version != "2.1.274" {
		t.Errorf("Version = %q, want 2.1.274", status.Version)
	}
	if !status.LoggedIn {
		t.Error("LoggedIn = false, want true")
	}
}

func TestCheckClaudeCodeSignedOut(t *testing.T) {
	cli := newStatusCLI(t, "2.1.274 (Claude Code)", `{"loggedIn":false}`)

	status := CheckClaudeCode(context.Background(), cli)
	if !status.Installed {
		t.Error("Installed = false, want true — the binary is there, only the session is not")
	}
	if status.LoggedIn {
		t.Error("LoggedIn = true, want false")
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
// auth status` also prints the account's email and organisation, and none of
// that may travel to the frontend.
func TestClaudeCodeStatusCarriesNoAccountDetails(t *testing.T) {
	cli := newStatusCLI(t, "2.1.274 (Claude Code)", `{"loggedIn":true,"email":"someone@example.com","orgName":"Someone's Org"}`)

	status := CheckClaudeCode(context.Background(), cli)
	rendered := status.Path + status.Version
	if strings.Contains(rendered, "someone@example.com") || strings.Contains(rendered, "Someone's Org") {
		t.Errorf("status leaked account details: %+v", status)
	}
}

// newStatusCLI writes a stub that answers `--version` and `auth status`.
func newStatusCLI(t *testing.T, version, authJSON string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub CLI is a shell script; the Windows probe path is covered by review, not by this test")
	}
	path := filepath.Join(t.TempDir(), "claude")
	script := "#!/bin/sh\ncase \"$1\" in\n" +
		"  --version) echo '" + version + "' ;;\n" +
		"  auth) echo '" + authJSON + "' ;;\n" +
		"  *) exit 1 ;;\nesac\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub CLI: %v", err)
	}
	return path
}
