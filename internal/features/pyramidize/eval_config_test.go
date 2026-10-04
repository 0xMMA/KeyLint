package pyramidize

import (
	"testing"

	"keylint/internal/llm"
)

// envFrom is a getenv over a fixed map, so these tests read nothing ambient.
func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// TestTheEvalRunsThroughTheCLIByDefault: every eval call goes through the
// owner's subscription unless a run says otherwise, and the model is the CLI
// alias users get — not a pinned ID.
func TestTheEvalRunsThroughTheCLIByDefault(t *testing.T) {
	provider, model := evalTarget(envFrom(nil))
	if provider != llm.ProviderClaudeCode {
		t.Errorf("provider = %q, want %q", provider, llm.ProviderClaudeCode)
	}
	if model != "sonnet" {
		t.Errorf("model = %q, want the CLI alias sonnet", model)
	}
}

// TestTheAPIIsStillOneFlagAway: --provider claude is how a run measures the API.
func TestTheAPIIsStillOneFlagAway(t *testing.T) {
	provider, model := evalTarget(envFrom(map[string]string{"EVAL_PROVIDER": llm.ProviderClaude}))
	if provider != llm.ProviderClaude {
		t.Errorf("provider = %q, want %q", provider, llm.ProviderClaude)
	}
	if want := llm.DefaultModel(llm.ProviderClaude, llm.FeaturePyramidize); model != want {
		t.Errorf("model = %q, want the API default %q", model, want)
	}

	_, model = evalTarget(envFrom(map[string]string{"EVAL_PROVIDER": llm.ProviderClaude, "EVAL_MODEL": "claude-opus-4-6"}))
	if model != "claude-opus-4-6" {
		t.Errorf("model = %q, want the explicit override", model)
	}
}

// TestACLIOnlyRunTakesNoKeyFromDotEnv: the CLI client strips a key from the
// child's environment anyway, but a key the run never loads cannot be the one
// that answered — or the one that got billed.
func TestACLIOnlyRunTakesNoKeyFromDotEnv(t *testing.T) {
	file := map[string]string{"ANTHROPIC_API_KEY": "sk-ant-file", "EVAL_VARIANT": "1"}

	got := evalEnvFromFile(file, envFrom(nil))
	if _, ok := got["ANTHROPIC_API_KEY"]; ok {
		t.Error("a run with no API provider loaded the key from .env")
	}
	if got["EVAL_VARIANT"] != "1" {
		t.Errorf("EVAL_VARIANT = %q, want the .env value filling an empty slot", got["EVAL_VARIANT"])
	}
}

// TestAnAPIRunStillTakesItsKey covers each way an API provider enters a run:
// the pipeline from the shell, the pipeline from .env, and the judge.
func TestAnAPIRunStillTakesItsKey(t *testing.T) {
	cases := map[string]struct {
		file map[string]string
		env  map[string]string
	}{
		"pipeline from the shell": {
			file: map[string]string{"ANTHROPIC_API_KEY": "sk-ant-file"},
			env:  map[string]string{"EVAL_PROVIDER": llm.ProviderClaude},
		},
		"pipeline from .env": {
			file: map[string]string{"ANTHROPIC_API_KEY": "sk-ant-file", "EVAL_PROVIDER": llm.ProviderClaude},
		},
		"judge only": {
			file: map[string]string{"ANTHROPIC_API_KEY": "sk-ant-file"},
			env:  map[string]string{"EVAL_JUDGE_PROVIDER": llm.ProviderClaude},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := evalEnvFromFile(tc.file, envFrom(tc.env))
			if got["ANTHROPIC_API_KEY"] != "sk-ant-file" {
				t.Errorf("ANTHROPIC_API_KEY = %q, want the .env key", got["ANTHROPIC_API_KEY"])
			}
		})
	}
}

// TestDotEnvNeverOverridesTheCommandLine: `--model X` once lost to a line in a
// file. Only credentials win over the shell.
func TestDotEnvNeverOverridesTheCommandLine(t *testing.T) {
	got := evalEnvFromFile(
		map[string]string{"EVAL_MODEL": "from-file", "EVAL_PROVIDER": llm.ProviderClaude, "ANTHROPIC_API_KEY": "file"},
		envFrom(map[string]string{"EVAL_MODEL": "from-flag", "EVAL_PROVIDER": llm.ProviderClaudeCode}),
	)
	if _, ok := got["EVAL_MODEL"]; ok {
		t.Errorf("EVAL_MODEL = %q, the command line's value must stand", got["EVAL_MODEL"])
	}
	// The shell said claude-code, so .env's claude must not pull a key in.
	if _, ok := got["ANTHROPIC_API_KEY"]; ok {
		t.Error("a key was loaded for a provider the command line overrode")
	}
}

// TestAnEmptyKeyInDotEnvDoesNotBlankTheShellsKey: .env.example ships the line
// empty, and a copy of it must not override a key the shell exports.
func TestAnEmptyKeyInDotEnvDoesNotBlankTheShellsKey(t *testing.T) {
	got := evalEnvFromFile(map[string]string{"ANTHROPIC_API_KEY": ""},
		envFrom(map[string]string{"EVAL_PROVIDER": llm.ProviderClaude}))
	if _, ok := got["ANTHROPIC_API_KEY"]; ok {
		t.Error("an empty .env key was set over the environment's")
	}
}

func TestTheCLIVersionIsOnlyAskedForWhenTheCLIRuns(t *testing.T) {
	if got := evalCLIVersion(llm.ProviderClaude, llm.ProviderOpenAI); got != "" {
		t.Errorf("evalCLIVersion without the CLI = %q, want empty", got)
	}
}

func TestOnlyEvalSettingsLeaveDotEnvForPyramidize(t *testing.T) {
	got := evalEnvFromFile(map[string]string{"EVAL_VARIANT": "1", "CLAUDE_CONFIG_DIR": "/elsewhere"}, envFrom(nil))
	if got["EVAL_VARIANT"] != "1" {
		t.Error("EVAL_VARIANT was dropped")
	}
	if _, ok := got["CLAUDE_CONFIG_DIR"]; ok {
		t.Error("CLAUDE_CONFIG_DIR left .env and would reach the CLI")
	}
}

func TestPyramidizeThinkingLabel(t *testing.T) {
	if pyramidizeThinking(llm.ProviderClaudeCode) != "on" || pyramidizeThinking(llm.ProviderClaude) != "n/a" {
		t.Error("thinking labels drifted")
	}
}
