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
