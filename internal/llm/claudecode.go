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
	"unicode/utf8"

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

// terminalReasonCompleted is what the CLI reports for a run that finished.
const terminalReasonCompleted = "completed"

// stopReasonMaxTokens is the one stop reason that means the answer was cut off.
const stopReasonMaxTokens = "max_tokens"

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
	// A pasted OAuth token stands in for the signed-in account just like a key.
	"CLAUDE_CODE_OAUTH_TOKEN",
	// Would answer with a different model than the one KeyLint asked for.
	"ANTHROPIC_MODEL",
	"CLAUDE_CODE_USE_BEDROCK",
	"CLAUDE_CODE_USE_VERTEX",
	"AWS_BEARER_TOKEN_BEDROCK",
}

// claudeCodeEnvelope is what `claude -p --output-format json` prints. Only the
// fields KeyLint acts on are listed.
type claudeCodeEnvelope struct {
	Result string `json:"result"`
	// StructuredOutput is the parsed object when --json-schema was passed. The
	// CLI fills both: result carries the same JSON as a string.
	StructuredOutput json.RawMessage `json:"structured_output"`
	IsError          bool            `json:"is_error"`
	TerminalReason   string          `json:"terminal_reason"`
	// StopReason is why the model stopped, as opposed to why the run ended.
	// Only "max_tokens" means a cut-off answer: the CLI sets this to values like
	// "tool_use" and "end_turn" on perfectly good runs, so it cannot be gated on
	// wholesale the way terminal_reason can.
	StopReason   string         `json:"stop_reason"`
	DurationMS   int64          `json:"duration_ms"`
	TotalCostUSD float64        `json:"total_cost_usd"`
	ModelUsage   map[string]any `json:"modelUsage"`
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

	// The CLI takes a full model ID as well as an alias, so this is a note and
	// not a rejection: a pinned ID is a deliberate choice, it just freezes the
	// generation where an alias follows it. The picker offers only aliases.
	if !IsClaudeCodeAlias(req.Model) {
		logger.Info("llm: claude code model is not one of the aliases",
			"feature", c.cfg.Feature, "model", req.Model, "aliases", "opus, sonnet, haiku")
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
	if len(req.JSONSchema) > 0 {
		// Compacted, because this goes on the command line and the CLI has no
		// --json-schema-file. A pretty-printed schema carries newlines, and
		// cmd.exe — which re-parses the line for the usual claude.cmd shim —
		// ends the command at the first one, truncating the schema mid-object.
		// Go's argument escaping handles quotes and backslashes but not
		// newlines; this is the same trap --system-prompt was moved off.
		var compact bytes.Buffer
		if err := json.Compact(&compact, req.JSONSchema); err != nil {
			return Response{}, fmt.Errorf("%s: invalid JSON schema: %w", name, err)
		}
		args = append(args, "--json-schema", compact.String())
	}
	if promptFile != "" {
		args = append(args, "--system-prompt-file", promptFile)
	}

	// The path carries the user's account name on Windows, so it stays out of
	// the log level people attach to bug reports.
	logger.Debug("llm: request", "feature", c.cfg.Feature, "provider", claudeCodeProvider.id,
		"path", logger.Redact(path), "model", req.Model,
		"system", logger.Redact(req.System), "user", logger.Redact(req.User))

	cmd := exec.CommandContext(ctx, path, args...)
	cliEnviron, removed := cliEnv()
	cmd.Env = cliEnviron
	cmd.Stdin = strings.NewReader(req.User)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = cliWaitDelay
	configureCLIProcess(cmd)

	// Names only — the values are credentials. One line per completion, so a bug
	// report shows why the CLI saw a different environment than KeyLint itself.
	if len(removed) > 0 {
		logger.Info("llm: credential variables removed for the claude code cli",
			"vars", strings.Join(removed, ", "))
	}

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
	// A run that ended badly still fills result, and pasting a half-written
	// answer over the user's selection is worse than saying nothing. "completed"
	// is what a finished run reports; the failure values are api_error,
	// model_error, prompt_too_long, aborted_streaming and the like. An
	// output-token cut-off is *not* among them — that is stop_reason, which the
	// CLI also sets to non-"end_turn" values on perfectly good runs, so gating
	// on it would reject answers that are fine. An older CLI leaves the field
	// empty and is unaffected.
	// The HTTP providers refuse a truncated answer; this path does the same
	// rather than silently differing.
	if env.StopReason == stopReasonMaxTokens {
		return Response{}, fmt.Errorf("%s: %s", name, outputLimitMessage)
	}

	if env.TerminalReason != "" && env.TerminalReason != terminalReasonCompleted {
		// The partial answer is the user's own text. It goes to Debug through
		// Redact, not into the error — an error string is formatted into Error
		// lines whatever the sensitive-logging setting says (#41).
		logger.Debug("llm: claude code stopped early", "feature", c.cfg.Feature,
			"terminal_reason", env.TerminalReason, "partial", logger.Redact(partialText(env)))
		return Response{}, fmt.Errorf("%s stopped before finishing (%s)", name, env.TerminalReason)
	}

	text := env.Result
	// With --json-schema the CLI parses the object itself; preferring it means a
	// stray fence or trailing note in result cannot reach the caller's parser.
	// A JSON null is four bytes, so a length check alone would hand the literal
	// string "null" to the caller's parser.
	if len(env.StructuredOutput) > 0 && !bytes.Equal(bytes.TrimSpace(env.StructuredOutput), []byte("null")) {
		text = string(env.StructuredOutput)
	}

	// An empty result would be pasted over the user's selection as nothing at
	// all, so treat it the way the HTTP providers treat an empty content block.
	if strings.TrimSpace(text) == "" {
		return Response{}, fmt.Errorf("%s returned an empty result", name)
	}

	logger.Info("llm: claude code call finished", "feature", c.cfg.Feature,
		"model", req.Model, "duration_ms", env.DurationMS, "total_cost_usd", env.TotalCostUSD)

	return Response{Text: text}, nil
}

// partialText summarises what a cut-off run produced, for the debug log only.
func partialText(env claudeCodeEnvelope) string {
	partial := strings.TrimSpace(env.Result)
	if partial == "" {
		return "no output"
	}
	const limit = 120
	if len(partial) <= limit {
		return partial
	}
	// Byte-slicing would split a multi-byte rune; the corpus this truncates is
	// largely German prose.
	cut := limit
	for cut > 0 && !utf8.ValidString(partial[:cut]) {
		cut--
	}
	return partial[:cut] + "…"
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
// point it at another account, plus the names of the variables it dropped. Both
// the completion call and the discovery probe use it, so what KeyLint detects
// and what it later runs see the same environment. See blockedCLIEnv.
func cliEnv() (env []string, removed []string) {
	parent := os.Environ()
	filtered := make([]string, 0, len(parent))
	for _, entry := range parent {
		name, _, found := strings.Cut(entry, "=")
		if found && isBlockedCLIEnv(name) {
			removed = append(removed, name)
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered, removed
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
	// The envelope is the last thing the CLI prints. Anything before it — an
	// update notice, a shim's warning — may itself contain a brace, so anchoring
	// on the first one loses a perfectly good envelope.
	if env, ok := decodeEnvelope(out); ok {
		return env, true
	}
	lines := bytes.Split(out, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		if env, ok := decodeEnvelope(lines[i]); ok {
			return env, true
		}
	}
	// A pretty-printed envelope spans lines; take everything from the last
	// opening brace that starts a line.
	if start := bytes.LastIndex(out, []byte("\n{")); start >= 0 {
		if env, ok := decodeEnvelope(out[start+1:]); ok {
			return env, true
		}
	}
	// Last resort, and the one the previous parser relied on: decode the first
	// object in the stream and ignore whatever follows it. This catches a
	// pretty-printed envelope with a notice printed after it, which none of the
	// strategies above can see.
	if start := bytes.IndexByte(out, '{'); start >= 0 {
		var env claudeCodeEnvelope
		if err := json.NewDecoder(bytes.NewReader(out[start:])).Decode(&env); err == nil {
			return env, true
		}
	}
	return claudeCodeEnvelope{}, false
}

// decodeEnvelope accepts a chunk only if it is a JSON object carrying at least
// one field the CLI always sets, so a stray object in a preamble is not
// mistaken for the envelope.
func decodeEnvelope(chunk []byte) (claudeCodeEnvelope, bool) {
	chunk = bytes.TrimSpace(chunk)
	if len(chunk) == 0 || chunk[0] != '{' {
		return claudeCodeEnvelope{}, false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(chunk, &probe); err != nil {
		return claudeCodeEnvelope{}, false
	}
	if _, ok := probe["result"]; !ok {
		if _, ok := probe["is_error"]; !ok {
			return claudeCodeEnvelope{}, false
		}
	}
	var env claudeCodeEnvelope
	if err := json.Unmarshal(chunk, &env); err != nil {
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
