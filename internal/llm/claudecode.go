package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"keylint/internal/logger"
)

// claudeCodeProvider carries the ID used in logs and the name used in errors.
// The name stays plain text: Anthropic's terms allow saying that KeyLint uses
// the installed Claude Code CLI, but not branding a feature with it.
var claudeCodeProvider = provider{id: ProviderClaudeCode, name: "Claude Code CLI"}

// notSignedInMessage tells the user the one thing that fixes this. Signing in
// happens in Anthropic's own flow, in their terminal — never inside KeyLint.
const notSignedInMessage = "Claude Code is installed but not signed in. Open a terminal, run `claude`, and sign in."

// cliWaitDelay bounds how long we wait for output pipes to drain after the
// process itself has been killed, so a stuck child cannot hang the caller.
const cliWaitDelay = 5 * time.Second

// blockedCLIEnv lists environment variables that would redirect the CLI away
// from the account the user signed in with. KeyLint itself reads
// ANTHROPIC_API_KEY for its BYOK providers, and the CLI treats that key as
// taking precedence over the claude.ai login — so inheriting it would silently
// bill a different account than the UI promises, and an expired key makes the
// CLI hang instead of failing. Stripping them keeps this provider what it says
// it is: the user's own subscription, and no credential passed along by us.
var blockedCLIEnv = []string{
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_BASE_URL",
	"CLAUDE_CODE_USE_BEDROCK",
	"CLAUDE_CODE_USE_VERTEX",
	"AWS_BEARER_TOKEN_BEDROCK",
}

// claudeCodeEnvelope is what `claude -p --output-format json` prints. Only the
// fields KeyLint acts on are listed.
type claudeCodeEnvelope struct {
	Result         string         `json:"result"`
	IsError        bool           `json:"is_error"`
	TerminalReason string         `json:"terminal_reason"`
	DurationMS     int64          `json:"duration_ms"`
	TotalCostUSD   float64        `json:"total_cost_usd"`
	ModelUsage     map[string]any `json:"modelUsage"`
}

type claudeCodeClient struct {
	cfg Config
}

func newClaudeCode(cfg Config) Client { return &claudeCodeClient{cfg: cfg} }

// Complete runs one print-mode call against the locally installed binary.
// The prompt goes in on stdin and the system prompt via a file, so neither ever
// reaches the command line.
func (c *claudeCodeClient) Complete(ctx context.Context, req Request) (Response, error) {
	name := claudeCodeProvider.name
	if req.Model == "" {
		return Response{}, fmt.Errorf("%s: model is required", name)
	}

	path := c.cfg.CLIPath
	if path == "" {
		located, err := LocateClaudeCode()
		if err != nil {
			return Response{}, err
		}
		path = located
	}

	// System prompts run to several kilobytes and are full of quotes and
	// newlines. On Windows the binary is usually claude.cmd, and cmd.exe re-parses
	// the command line with quoting rules Go does not escape for and a hard 8191
	// character limit — so passing the prompt as an argument would corrupt it.
	// A file sidesteps the command line entirely, on every platform.
	promptFile, cleanup, err := writeSystemPromptFile(req.System)
	if err != nil {
		return Response{}, fmt.Errorf("%s: %w", name, err)
	}
	defer cleanup()

	// --bare looks right here but skips credential reads, which makes a
	// signed-in user look signed out. Never add it.
	args := []string{
		"-p",
		"--model", req.Model,
		"--output-format", "json",
		"--no-session-persistence",
		"--tools", "",
		"--strict-mcp-config",
		"--setting-sources", "",
		"--disable-slash-commands",
	}
	if promptFile != "" {
		args = append(args, "--system-prompt-file", promptFile)
	}

	logger.Debug("llm: request", "feature", c.cfg.Feature, "provider", claudeCodeProvider.id,
		"path", path, "model", req.Model,
		"system", logger.Redact(req.System), "user", logger.Redact(req.User))

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = cliEnv()
	cmd.Stdin = strings.NewReader(req.User)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = cliWaitDelay
	configureCLIProcess(cmd)

	runErr := cmd.Run()
	out := stdout.Bytes()

	logger.Debug("llm: response", "feature", c.cfg.Feature, "provider", claudeCodeProvider.id,
		"exit_err", runErr, "stdout", logger.Redact(string(out)), "stderr", logger.Redact(stderr.String()))

	// A cancelled or timed-out context is the caller's own doing, so report it
	// as such and keep it unwrappable. stderr rides along because it is often
	// the only place the CLI says why it went quiet.
	if ctxErr := ctx.Err(); ctxErr != nil {
		if detail := lastNonEmptyLine(stderr.String()); detail != "" {
			return Response{}, fmt.Errorf("%s request failed (%s): %w", name, detail, ctxErr)
		}
		return Response{}, fmt.Errorf("%s request failed: %w", name, ctxErr)
	}

	env, parsed := parseClaudeCodeEnvelope(out)

	if parsed && env.IsError {
		return Response{}, fmt.Errorf("%s: %s", name, claudeCodeFailure(env))
	}
	if runErr != nil {
		if parsed && env.Result != "" {
			return Response{}, fmt.Errorf("%s: %s", name, claudeCodeFailure(env))
		}
		return Response{}, fmt.Errorf("%s failed: %s", name, cliFailureDetail(runErr, stderr.String()))
	}
	if !parsed {
		return Response{}, fmt.Errorf("%s unexpected response: %s", name, out)
	}
	// An empty result would be pasted over the user's selection as nothing at
	// all, so treat it the way the HTTP providers treat an empty content block.
	if strings.TrimSpace(env.Result) == "" {
		return Response{}, fmt.Errorf("%s returned an empty result", name)
	}

	logger.Info("llm: claude code call finished", "feature", c.cfg.Feature,
		"model", req.Model, "duration_ms", env.DurationMS, "total_cost_usd", env.TotalCostUSD)

	return Response{Text: env.Result}, nil
}

// writeSystemPromptFile stores the system prompt where the CLI can read it.
// It returns an empty path when there is no system prompt to pass.
func writeSystemPromptFile(systemPrompt string) (path string, cleanup func(), err error) {
	if systemPrompt == "" {
		return "", func() {}, nil
	}
	// 0600: the prompt is the user's own text and nobody else's business.
	file, err := os.CreateTemp("", "keylint-system-prompt-*.txt")
	if err != nil {
		return "", func() {}, fmt.Errorf("could not stage the system prompt: %w", err)
	}
	remove := func() { _ = os.Remove(file.Name()) }
	if _, err := file.WriteString(systemPrompt); err != nil {
		_ = file.Close()
		remove()
		return "", func() {}, fmt.Errorf("could not write the system prompt: %w", err)
	}
	if err := file.Close(); err != nil {
		remove()
		return "", func() {}, fmt.Errorf("could not write the system prompt: %w", err)
	}
	return file.Name(), remove, nil
}

// cliEnv returns the environment for the CLI: ours, minus anything that would
// point it at another account. See blockedCLIEnv.
func cliEnv() []string {
	parent := os.Environ()
	filtered := make([]string, 0, len(parent))
	for _, entry := range parent {
		name, _, found := strings.Cut(entry, "=")
		if found && isBlockedCLIEnv(name) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

// isBlockedCLIEnv matches case-insensitively, because Windows environment
// variable names are.
func isBlockedCLIEnv(name string) bool {
	for _, blocked := range blockedCLIEnv {
		if strings.EqualFold(name, blocked) {
			return true
		}
	}
	return false
}

// parseClaudeCodeEnvelope decodes the JSON envelope, tolerating anything the
// CLI or an npm shim printed before it (update notices, deprecation warnings).
func parseClaudeCodeEnvelope(out []byte) (claudeCodeEnvelope, bool) {
	start := bytes.IndexByte(out, '{')
	if start < 0 {
		return claudeCodeEnvelope{}, false
	}
	var env claudeCodeEnvelope
	if err := json.NewDecoder(bytes.NewReader(out[start:])).Decode(&env); err != nil {
		return claudeCodeEnvelope{}, false
	}
	return env, true
}

// claudeCodeFailure turns an error envelope into something the user can act on.
func claudeCodeFailure(env claudeCodeEnvelope) string {
	if isNotSignedIn(env.Result) || isNotSignedIn(env.TerminalReason) {
		return notSignedInMessage
	}
	if env.Result != "" {
		return env.Result
	}
	if env.TerminalReason != "" {
		return env.TerminalReason
	}
	return "the CLI reported an error without a reason"
}

// isNotSignedIn recognises the CLI's logged-out wording.
func isNotSignedIn(s string) bool {
	lowered := strings.ToLower(s)
	return strings.Contains(lowered, "not logged in") ||
		strings.Contains(lowered, "please run /login") ||
		strings.Contains(lowered, "not authenticated")
}

// cliFailureDetail prefers the last line of stderr, which is where a failing
// CLI puts its diagnosis — the first line is often an unrelated update notice
// from the npm shim — and falls back to the exec error.
func cliFailureDetail(runErr error, stderr string) string {
	if line := lastNonEmptyLine(stderr); line != "" {
		return line
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return fmt.Sprintf("exited with status %d and no output", exitErr.ExitCode())
	}
	return runErr.Error()
}

// lastNonEmptyLine returns the final line with content, or "".
func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if trimmed := strings.TrimSpace(lines[i]); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
