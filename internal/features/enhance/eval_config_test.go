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

// TestTheFixJudgesArePinnedThroughTheCLI: same instruments as the Pyramidize
// suite — two judges through the CLI, full model IDs, and an honest null
// temperature on each. The pipeline's provider and model reach neither.
func TestTheFixJudgesArePinnedThroughTheCLI(t *testing.T) {
	judges, err := JudgesFromEnv(envFrom(map[string]string{"EVAL_PROVIDER": "openai", "EVAL_MODEL": "gpt-4.1"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(judges) != 2 || judges[0].Model != "claude-haiku-5-5" || judges[1].Model != "claude-sonnet-5-5" {
		t.Fatalf("judges = %+v, want claude-haiku-5-5 and claude-sonnet-5-5", judges)
	}
	for _, cfg := range judges {
		if cfg.Provider != llm.ProviderClaudeCode {
			t.Errorf("%s: provider = %q, want %q", cfg.Model, cfg.Provider, llm.ProviderClaudeCode)
		}
		if cfg.Temperature != nil {
			t.Errorf("%s: temperature = %v, want nil — the CLI cannot pin it", cfg.Model, *cfg.Temperature)
		}
		if cfg.TemperatureNote == "" {
			t.Errorf("%s: an unpinned temperature carries no note saying why", cfg.Model)
		}
		data, _ := json.Marshal(cfg)
		var back map[string]any
		_ = json.Unmarshal(data, &back)
		if v, ok := back["temperature"]; !ok || v != nil {
			t.Errorf("%s: temperature in JSON = %v (present=%v), want null", cfg.Model, v, ok)
		}
	}
}

// TestAnAPIFixJudgeIsPinnedOnlyWhereTheModelTakesIt: on the API the 5.5
// judges reject a non-default temperature and record null with a note; the
// Sonnet 4.5 bridge judge keeps the pin.
func TestAnAPIFixJudgeIsPinnedOnlyWhereTheModelTakesIt(t *testing.T) {
	judges, err := JudgesFromEnv(envFrom(map[string]string{"EVAL_JUDGE_PROVIDER": llm.ProviderClaude}))
	if err != nil {
		t.Fatal(err)
	}
	for _, cfg := range judges {
		if cfg.Temperature != nil || cfg.TemperatureNote == "" {
			t.Errorf("%s: temperature %v, note %q — want null with a note", cfg.Model, cfg.Temperature, cfg.TemperatureNote)
		}
	}
	judges, err = JudgesFromEnv(envFrom(map[string]string{
		"EVAL_JUDGE_PROVIDER": llm.ProviderClaude, "EVAL_JUDGES": "claude-sonnet-4-5-20250929"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg := judges[0]; cfg.Temperature == nil || *cfg.Temperature != 0 || cfg.TemperatureNote != "" {
		t.Errorf("bridge judge: temperature %v, note %q — want a pinned 0", cfg.Temperature, cfg.TemperatureNote)
	}
}

// TestTheFixJudgeSetCanBeChosen: EVAL_JUDGES picks the set (a bridge run adds
// the old Sonnet 4.5 judge), EVAL_JUDGE_MODEL is a set of one, and a set with a
// hole, a duplicate or both variables is refused.
func TestTheFixJudgeSetCanBeChosen(t *testing.T) {
	judges, err := JudgesFromEnv(envFrom(map[string]string{"EVAL_JUDGES": "claude-haiku-5-5,claude-sonnet-5-5, claude-sonnet-4-5-20250929"}))
	if err != nil || len(judges) != 3 || judges[2].Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("EVAL_JUDGES: %+v, %v", judges, err)
	}
	judges, err = JudgesFromEnv(envFrom(map[string]string{"EVAL_JUDGE_MODEL": "claude-sonnet-4-5-20250929"}))
	if err != nil || len(judges) != 1 || judges[0].Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("EVAL_JUDGE_MODEL: %+v, %v", judges, err)
	}
	for _, env := range []map[string]string{
		{"EVAL_JUDGES": "claude-haiku-5-5,"},
		{"EVAL_JUDGES": "claude-haiku-5-5,claude-haiku-5-5"},
		{"EVAL_JUDGES": "claude-haiku-5-5", "EVAL_JUDGE_MODEL": "claude-sonnet-5-5"},
	} {
		if judges, err := JudgesFromEnv(envFrom(env)); err == nil {
			t.Errorf("%v was accepted: %+v", env, judges)
		}
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

// TestFixThinkingLabelFollowsTheService: the label is what Fix does about
// reasoning on that route, so a run records it rather than "default".
func TestFixThinkingLabelFollowsTheService(t *testing.T) {
	cases := []struct{ provider, model, want string }{
		// The CLI thinks by default, and Fix does not switch it off there.
		{llm.ProviderClaudeCode, "haiku", "on"},
		{llm.ProviderClaudeCode, "sonnet", "on"},
		// Fix asks a fast-tier API model not to reason (Request.NoThinking).
		{llm.ProviderClaude, "claude-haiku-5-5", "off"},
		{llm.ProviderOpenAI, "gpt-6-luna", "off"},
		{llm.ProviderClaude, "claude-sonnet-5-5", "n/a"},
	}
	for _, tc := range cases {
		if got := fixThinkingLabel(tc.provider, tc.model); got != tc.want {
			t.Errorf("fixThinkingLabel(%s, %s) = %q, want %q", tc.provider, tc.model, got, tc.want)
		}
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
