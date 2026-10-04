package enhance

import (
	"encoding/json"
	"testing"

	"keylint/internal/llm"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// TestTheFixEvalRunsThroughTheCLIByDefault: the subscription, and the alias a
// user of the silent fix gets.
func TestTheFixEvalRunsThroughTheCLIByDefault(t *testing.T) {
	provider, model := evalTarget(envFrom(nil))
	if provider != llm.ProviderClaudeCode {
		t.Errorf("provider = %q, want %q", provider, llm.ProviderClaudeCode)
	}
	if model != "haiku" {
		t.Errorf("model = %q, want the CLI alias haiku", model)
	}

	provider, model = evalTarget(envFrom(map[string]string{"EVAL_PROVIDER": llm.ProviderClaude}))
	if provider != llm.ProviderClaude || model != llm.DefaultModel(llm.ProviderClaude, llm.FeatureFix) {
		t.Errorf("--provider claude gave %s/%s, want the API and its default model", provider, model)
	}
}

// TestTheFixJudgeIsPinnedThroughTheCLI: same instrument as the Pyramidize
// suite — the CLI, the dated snapshot, and an honest null temperature.
func TestTheFixJudgeIsPinnedThroughTheCLI(t *testing.T) {
	cfg := JudgeConfigFromEnv(envFrom(map[string]string{"EVAL_PROVIDER": "openai", "EVAL_MODEL": "gpt-4.1"}))
	if cfg.Provider != llm.ProviderClaudeCode || cfg.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("judge = %s/%s, want claude-code/claude-sonnet-4-5-20250929 — the pipeline must not reach it", cfg.Provider, cfg.Model)
	}
	if cfg.Temperature != nil {
		t.Errorf("temperature = %v, want nil — the CLI cannot pin it", *cfg.Temperature)
	}
	if cfg.TemperatureNote == "" {
		t.Error("an unpinned temperature carries no note saying why")
	}
	data, _ := json.Marshal(cfg)
	var back map[string]any
	_ = json.Unmarshal(data, &back)
	if v, ok := back["temperature"]; !ok || v != nil {
		t.Errorf("temperature in JSON = %v (present=%v), want null", v, ok)
	}
}

func TestAnAPIFixJudgeIsPinnedAtZero(t *testing.T) {
	cfg := JudgeConfigFromEnv(envFrom(map[string]string{"EVAL_JUDGE_PROVIDER": llm.ProviderClaude}))
	if cfg.Temperature == nil || *cfg.Temperature != 0 {
		t.Fatalf("temperature = %v, want a pinned 0", cfg.Temperature)
	}
	if cfg.TemperatureNote != "" {
		t.Errorf("a pinned temperature carries a note: %q", cfg.TemperatureNote)
	}
}

func TestTheFixEvalTakesAKeyOnlyForAnAPIProvider(t *testing.T) {
	file := map[string]string{"ANTHROPIC_API_KEY": "sk-ant-file"}
	if _, ok := evalEnvFromFile(file, envFrom(nil))["ANTHROPIC_API_KEY"]; ok {
		t.Error("a CLI-only run loaded the key from .env")
	}
	for _, env := range []map[string]string{
		{"EVAL_PROVIDER": llm.ProviderClaude},
		{"EVAL_JUDGE_PROVIDER": llm.ProviderClaude},
	} {
		if got := evalEnvFromFile(file, envFrom(env))["ANTHROPIC_API_KEY"]; got != "sk-ant-file" {
			t.Errorf("with %v the key was %q, want it loaded", env, got)
		}
	}
}

// TestEvalThinkingNamesTheEffectiveSetting: the label is what ran, never
// "default", so changing fixThinking makes old runs read "not comparable".
func TestEvalThinkingNamesTheEffectiveSetting(t *testing.T) {
	cases := []struct {
		provider, env string
		thinking      bool
		label         string
	}{
		{llm.ProviderClaudeCode, "off", false, "off"},
		{llm.ProviderClaudeCode, "on", true, "on"},
		{llm.ProviderClaudeCode, "", fixThinking, map[bool]string{true: "on", false: "off"}[fixThinking]},
		{llm.ProviderClaude, "off", false, "n/a"},
	}
	for _, tc := range cases {
		thinking, label, err := evalThinking(tc.provider, envFrom(map[string]string{"EVAL_THINKING": tc.env}))
		if err != nil {
			t.Fatalf("%v: %v", tc, err)
		}
		if thinking != tc.thinking || label != tc.label {
			t.Errorf("%s/%q = %v/%q, want %v/%q", tc.provider, tc.env, thinking, label, tc.thinking, tc.label)
		}
	}
	if _, _, err := evalThinking(llm.ProviderClaudeCode, envFrom(map[string]string{"EVAL_THINKING": "maybe"})); err == nil {
		t.Error("an unknown EVAL_THINKING was accepted")
	}
}

func TestCLIVersionSpan(t *testing.T) {
	if got := cliVersionSpan("2.1.289", "2.1.289"); got != "2.1.289" {
		t.Errorf("same version = %q", got)
	}
	if got := cliVersionSpan("2.1.289", "2.1.290"); got != "2.1.289->2.1.290" {
		t.Errorf("upgrade mid-run = %q", got)
	}
}

// TestOnlyEvalSettingsLeaveDotEnv: the test process's environment is the CLI's
// environment. A stray line in .env must not reach it.
func TestOnlyEvalSettingsLeaveDotEnv(t *testing.T) {
	got := evalEnvFromFile(map[string]string{
		"EVAL_SPLIT":                "tune",
		"KEYLINT_PYRAMIDIZE_SCHEMA": "1",
		"MAX_THINKING_TOKENS":       "0",
		"CLAUDE_CONFIG_DIR":         "/elsewhere",
	}, envFrom(nil))
	for _, k := range []string{"EVAL_SPLIT", "KEYLINT_PYRAMIDIZE_SCHEMA"} {
		if _, ok := got[k]; !ok {
			t.Errorf("%s was dropped", k)
		}
	}
	for _, k := range []string{"MAX_THINKING_TOKENS", "CLAUDE_CONFIG_DIR"} {
		if _, ok := got[k]; ok {
			t.Errorf("%s left .env and would reach the CLI", k)
		}
	}
}
