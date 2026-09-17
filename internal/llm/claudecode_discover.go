package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"keylint/internal/logger"
)

// claudeCodeProbeTimeout bounds the two short probes behind a status check, so
// a wedged binary cannot stall the settings screen or the welcome wizard.
const claudeCodeProbeTimeout = 10 * time.Second

// ClaudeCodeStatus is what KeyLint can tell about the locally installed CLI.
// It deliberately carries no account details: `claude auth status` also prints
// the user's email and organisation, and none of that is KeyLint's business.
type ClaudeCodeStatus struct {
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
	Version   string `json:"version"`
	LoggedIn  bool   `json:"loggedIn"`
}

// LocateClaudeCode finds the installed binary: PATH first, then the places the
// official installers use. A desktop-launched app often has a shorter PATH than
// a terminal, so PATH alone is not enough.
func LocateClaudeCode() (string, error) {
	if path, err := exec.LookPath("claude"); err == nil {
		return path, nil
	}
	for _, candidate := range claudeCodeCandidates() {
		if isExecutable(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s: not found. Install Claude Code, or pick a provider with an API key in Settings", claudeCodeProvider.name)
}

// claudeCodeCandidates lists the per-platform install locations to probe.
func claudeCodeCandidates() []string {
	if runtime.GOOS == "windows" {
		var paths []string
		if appData := os.Getenv("APPDATA"); appData != "" {
			paths = append(paths, filepath.Join(appData, "npm", "claude.cmd"))
		}
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, "Programs", "claude", "claude.exe"))
		}
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			paths = append(paths, filepath.Join(userProfile, ".local", "bin", "claude.exe"))
		}
		return paths
	}

	paths := []string{"/usr/local/bin/claude", "/opt/homebrew/bin/claude"}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".local", "bin", "claude"),
			filepath.Join(home, ".npm-global", "bin", "claude"),
		)
	}
	return paths
}

// isExecutable reports whether path is a regular file KeyLint could run.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	// Windows decides by extension, not by a permission bit.
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode().Perm()&0111 != 0
}

// CheckClaudeCode reports whether the CLI is installed and signed in. cliPath
// overrides discovery; empty means locate it. It never returns an error: "not
// installed" is an answer the UI shows, not a failure.
func CheckClaudeCode(ctx context.Context, cliPath string) ClaudeCodeStatus {
	path := cliPath
	if path == "" {
		located, err := LocateClaudeCode()
		if err != nil {
			logger.Info("llm: claude code not found")
			return ClaudeCodeStatus{}
		}
		path = located
	}

	status := ClaudeCodeStatus{Installed: true, Path: path}
	status.Version = claudeCodeVersion(ctx, path)
	status.LoggedIn = claudeCodeLoggedIn(ctx, path)

	logger.Info("llm: claude code status", "path", status.Path, "version", status.Version, "logged_in", status.LoggedIn)
	return status
}

// claudeCodeVersion reads `claude --version`, e.g. "2.1.274 (Claude Code)".
func claudeCodeVersion(ctx context.Context, path string) string {
	out, err := runClaudeCode(ctx, path, "--version")
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if version, _, found := strings.Cut(line, " "); found {
		return version
	}
	return line
}

// claudeCodeLoggedIn reads `claude auth status`. The command also prints the
// account's email and organisation, so the output is never logged and only the
// login flag is kept.
func claudeCodeLoggedIn(ctx context.Context, path string) bool {
	out, err := runClaudeCode(ctx, path, "auth", "status")
	if err != nil {
		return false
	}
	var parsed struct {
		LoggedIn bool `json:"loggedIn"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return false
	}
	return parsed.LoggedIn
}

// runClaudeCode runs a short, side-effect-free probe and returns its stdout.
func runClaudeCode(ctx context.Context, path string, args ...string) ([]byte, error) {
	probeCtx, cancel := context.WithTimeout(ctx, claudeCodeProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, path, args...)
	cmd.WaitDelay = cliWaitDelay
	configureCLIProcess(cmd)
	return cmd.Output()
}
