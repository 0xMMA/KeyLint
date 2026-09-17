package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
// The prompt goes in on stdin so it never appears in the process table.
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
		"--system-prompt", req.System,
	}

	logger.Debug("llm: request", "feature", c.cfg.Feature, "provider", claudeCodeProvider.id,
		"path", path, "model", req.Model,
		"system", logger.Redact(req.System), "user", logger.Redact(req.User))

	cmd := exec.CommandContext(ctx, path, args...)
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
	// as such rather than as a CLI failure, and keep it unwrappable.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Response{}, fmt.Errorf("%s request failed: %w", name, ctxErr)
	}

	var env claudeCodeEnvelope
	parsed := json.Unmarshal(out, &env) == nil

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

	logger.Info("llm: claude code call finished", "feature", c.cfg.Feature,
		"model", req.Model, "duration_ms", env.DurationMS, "total_cost_usd", env.TotalCostUSD)

	return Response{Text: env.Result}, nil
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

// cliFailureDetail prefers the first line of stderr, which is where the CLI
// puts its own diagnosis, and falls back to the exec error.
func cliFailureDetail(runErr error, stderr string) string {
	for _, line := range strings.Split(stderr, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return fmt.Sprintf("exited with status %d and no output", exitErr.ExitCode())
	}
	return runErr.Error()
}
