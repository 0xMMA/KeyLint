package pyramidize

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// scripts/eval.sh decides which providers a run uses, which credentials it
// loads and whether the CLI is signed in — before any model is called. These
// run it with --dry-run against .env fixtures, so that logic is covered without
// spending a call. No eval build tag: this runs in the normal suite.

// evalShRun copies eval.sh into a fresh git repository with the given .env (or
// none when dotenv is nil), puts a stub `claude` first on PATH, and runs it.
// The stub prints authJSON for `claude auth status` and records its env.
func evalShRun(t *testing.T, dotenv *string, authJSON string, env []string, args ...string) (out string, code int, stubEnv string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("eval.sh is bash; CI runs it on Linux")
	}
	for _, tool := range []string{"bash", "git", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	src, err := filepath.Abs(filepath.Join("..", "..", "..", "scripts", "eval.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(repo, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "scripts", "eval.sh"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	if dotenv != nil {
		if err := os.WriteFile(filepath.Join(repo, ".env"), []byte(*dotenv), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	bin := t.TempDir()
	envFile := filepath.Join(bin, "claude.env")
	stub := "#!/usr/bin/env bash\nenv > " + envFile + "\nprintf '%s\\n' '" + authJSON + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(filepath.Join(repo, "scripts", "eval.sh"), append([]string{"--dry-run"}, args...)...)
	cmd.Dir = repo
	// A clean environment: nothing from the developer's shell decides the case.
	cmd.Env = append([]string{
		"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
	}, env...)
	raw, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running eval.sh: %v", err)
	}
	recorded, _ := os.ReadFile(envFile)
	return string(raw), code, string(recorded)
}

func dryRunLine(t *testing.T, out string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "dry-run ") {
			return line
		}
	}
	t.Fatalf("no dry-run line in output:\n%s", out)
	return ""
}

const signedIn = `{"loggedIn":true}`

// TestEvalShSurvivesADotEnvWithoutProviderLines: a .env that holds only a key —
// the owner's, and .env.example — once made eval.sh exit 1 silently under
// `set -e`, and 1 means "regression".
func TestEvalShSurvivesADotEnvWithoutProviderLines(t *testing.T) {
	for name, dotenv := range map[string]*string{
		"key only":     ptr("ANTHROPIC_API_KEY=sk-ant-file\n"),
		"example copy": ptr("# comment\n# ANTHROPIC_API_KEY=\n"),
		"no .env":      nil,
	} {
		t.Run(name, func(t *testing.T) {
			out, code, _ := evalShRun(t, dotenv, signedIn, nil)
			if code != 0 {
				t.Fatalf("exit = %d, want 0\n%s", code, out)
			}
			if got, want := dryRunLine(t, out), "dry-run pipeline=claude-code judge=claude-code keys=none"; got != want {
				t.Errorf("got  %q\nwant %q", got, want)
			}
		})
	}
}

// TestEvalShReadsDotEnvTheWayGoDoes: `export`, quotes and inline comments are
// what godotenv accepts, so the shell must read them the same way or the two
// halves of one run disagree about which provider it uses.
func TestEvalShReadsDotEnvTheWayGoDoes(t *testing.T) {
	dotenv := "export EVAL_PROVIDER=\"claude\"\n" +
		"EVAL_JUDGE_PROVIDER='claude' # back to the API judge\n" +
		"ANTHROPIC_API_KEY=\"sk-ant-quoted\"\r\n"
	out, code, _ := evalShRun(t, &dotenv, signedIn, nil)
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	if got, want := dryRunLine(t, out), "dry-run pipeline=claude judge=claude keys=ANTHROPIC_API_KEY"; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if !strings.Contains(out, "Judge:    claude / <pinned> @ temp 0") {
		t.Errorf("header does not show the API judge at temperature 0:\n%s", out)
	}
}

// TestEvalShKeepsTheShellsKeyOverAnEmptyLine: an empty value copied from
// .env.example must not count as a key.
func TestEvalShKeepsTheShellsKeyOverAnEmptyLine(t *testing.T) {
	dotenv := "ANTHROPIC_API_KEY=\n"
	out, code, _ := evalShRun(t, &dotenv, signedIn, []string{"EVAL_PROVIDER=claude"})
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	if got := dryRunLine(t, out); !strings.HasSuffix(got, "keys=none") {
		t.Errorf("an empty .env key was loaded: %q", got)
	}
}

// TestEvalShChecksSignInAsTheEvalWillRun: the pre-check strips what the Go
// client strips (blockedCLIEnv), or it could report a signed-in API-key session
// that every eval call — which never sees that key — then fails.
func TestEvalShChecksSignInAsTheEvalWillRun(t *testing.T) {
	blocked := []string{
		"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL", "CLAUDE_CODE_OAUTH_TOKEN",
		"ANTHROPIC_MODEL", "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "AWS_BEARER_TOKEN_BEDROCK",
	}
	var env []string
	for _, name := range blocked {
		env = append(env, name+"=from-the-shell")
	}
	out, code, stubEnv := evalShRun(t, nil, signedIn, env)
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	if stubEnv == "" {
		t.Fatal("the sign-in check never ran")
	}
	for _, name := range blocked {
		if strings.Contains(stubEnv, name+"=") {
			t.Errorf("%s reached `claude auth status`", name)
		}
	}

	out, code, _ = evalShRun(t, nil, `{"loggedIn":false}`, nil)
	if code != 3 {
		t.Errorf("signed out: exit = %d, want 3\n%s", code, out)
	}
}

func TestEvalShRejectsThinkingForPyramidize(t *testing.T) {
	out, code, _ := evalShRun(t, nil, signedIn, nil, "--thinking", "off")
	if code != 3 {
		t.Errorf("exit = %d, want 3 — Pyramidize thinking is left to the provider\n%s", code, out)
	}
	out, code, _ = evalShRun(t, nil, signedIn, nil, "--suite", "fix", "--thinking", "off")
	if code != 0 || !strings.Contains(out, "Thinking: off") {
		t.Errorf("fix --thinking off: exit %d\n%s", code, out)
	}
}

func ptr(s string) *string { return &s }
