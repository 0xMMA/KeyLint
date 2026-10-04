package pyramidize

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
)

// The judge is the instrument. These pin the properties that make its numbers
// comparable between runs — all without an API call, so they run in the normal
// suite rather than behind the eval tag.

// TestTheJudgeIsPinnedAndSeparateFromThePipeline: a pipeline under test changes,
// the thing measuring it must not. If the judge followed the pipeline's model,
// a Sonnet run and an Opus run could not go in the same table.
func TestTheJudgeIsPinnedAndSeparateFromThePipeline(t *testing.T) {
	t.Setenv("EVAL_JUDGE_PROVIDER", "")
	t.Setenv("EVAL_JUDGE_MODEL", "")
	t.Setenv("EVAL_PROVIDER", "openai")
	t.Setenv("EVAL_MODEL", "gpt-4.1")

	cfg := JudgeConfigFromEnv()

	if cfg.Provider != defaultJudgeProvider || cfg.Model != defaultJudgeModel {
		t.Errorf("judge = %s/%s, want the pinned %s/%s — the pipeline's provider must not reach it",
			cfg.Provider, cfg.Model, defaultJudgeProvider, defaultJudgeModel)
	}
}

// TestTheDefaultJudgeRunsThroughTheCLIAndSaysItIsUnpinned: the owner moved every
// eval call to the subscription, and the CLI has no temperature flag. Recording
// a 0 that never reached the model would claim a repeatability the instrument
// does not have.
func TestTheDefaultJudgeRunsThroughTheCLIAndSaysItIsUnpinned(t *testing.T) {
	t.Setenv("EVAL_JUDGE_PROVIDER", "")
	t.Setenv("EVAL_JUDGE_MODEL", "")

	cfg := JudgeConfigFromEnv()
	if cfg.Provider != llm.ProviderClaudeCode {
		t.Errorf("judge provider = %q, want %q", cfg.Provider, llm.ProviderClaudeCode)
	}
	if cfg.Temperature != nil {
		t.Errorf("temperature = %v, want nil — the CLI cannot pin it", *cfg.Temperature)
	}
	if cfg.TemperatureNote == "" {
		t.Error("an unpinned temperature carries no note saying why")
	}

	// And it has to reach summary.json as null, not as a missing field or a 0:
	// eval-aggregate.sh keys on it.
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if v, ok := back["temperature"]; !ok || v != nil {
		t.Errorf("temperature in JSON = %v (present=%v), want null", v, ok)
	}
}

// TestAnAPIJudgeIsStillPinnedAtZero: EVAL_JUDGE_PROVIDER=claude is the way back
// to the API judge, and there the pin still applies.
func TestAnAPIJudgeIsStillPinnedAtZero(t *testing.T) {
	t.Setenv("EVAL_JUDGE_PROVIDER", llm.ProviderClaude)
	t.Setenv("EVAL_JUDGE_MODEL", "")

	cfg := JudgeConfigFromEnv()
	if cfg.Temperature == nil || *cfg.Temperature != 0 {
		t.Fatalf("temperature = %v, want a pinned 0", cfg.Temperature)
	}
	if cfg.TemperatureNote != "" {
		t.Errorf("a pinned temperature carries a note: %q", cfg.TemperatureNote)
	}
	if cfg.Model != defaultJudgeModel {
		t.Errorf("model = %q, want the pinned snapshot", cfg.Model)
	}
}

// TestTheJudgeModelIsADatedSnapshot: #53 suspected the March baseline had
// drifted under an alias. An alias is what an instrument must not be.
func TestTheJudgeModelIsADatedSnapshot(t *testing.T) {
	// A dated Anthropic ID ends in -YYYYMMDD.
	const datePart = 8
	id := defaultJudgeModel
	if len(id) < datePart+1 || id[len(id)-datePart-1] != '-' {
		t.Fatalf("judge model %q is not a dated snapshot", id)
	}
	for _, r := range id[len(id)-datePart:] {
		if r < '0' || r > '9' {
			t.Fatalf("judge model %q does not end in a date", id)
		}
	}
}

// TestAnOverrideIsStillPossibleAndVisible: pinning must not make the judge
// unchangeable, only deliberate — and summary.json records what ran.
func TestAnOverrideIsStillPossibleAndVisible(t *testing.T) {
	t.Setenv("EVAL_JUDGE_PROVIDER", "openai")
	t.Setenv("EVAL_JUDGE_MODEL", "gpt-4.1")

	cfg := JudgeConfigFromEnv()
	if cfg.Provider != "openai" || cfg.Model != "gpt-4.1" {
		t.Errorf("judge = %s/%s, want the override", cfg.Provider, cfg.Model)
	}

	// It has to survive the round trip into summary.json, or a run judged by
	// something else would look like one that was not.
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var back JudgeConfig
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, cfg) {
		t.Errorf("round trip = %+v, want %+v", back, cfg)
	}
}

// TestTheJudgeSendsItsTemperatureAndSchema walks the real call path with a fake
// provider client: the pin is worth nothing if it stops at the struct.
func TestTheJudgeSendsItsTemperatureAndSchema(t *testing.T) {
	// The API judge: the one provider path where a temperature can be pinned.
	t.Setenv("EVAL_JUDGE_PROVIDER", llm.ProviderClaude)
	t.Setenv("EVAL_JUDGE_MODEL", "")
	svc, rec := newTestService()
	rec.client.reply = `{"pyramidStructure":0.8,"clarity":0.8,"completeness":0.8,"tonePreservation":0.8,"overall":0.8,"rationale":"ok"}`
	settingsSvc := settings.NewServiceFrom(settings.Default(), func(string) string { return "key" })

	// Through RunJudge itself, not a hand-built aiOpts: the pin is worth
	// nothing if RunJudge stops applying it.
	if _, err := svc.runJudge(settingsSvc, JudgeConfigFromEnv(), "raw", "baseline", "candidate"); err != nil {
		t.Fatalf("RunJudge: %v", err)
	}

	got := rec.client.gotRequest
	if got.Temperature == nil {
		t.Fatal("no temperature reached the provider")
	}
	if *got.Temperature != 0 {
		t.Errorf("temperature = %v, want 0", *got.Temperature)
	}
	if got.Model != defaultJudgeModel {
		t.Errorf("model = %q, want the pinned judge model", got.Model)
	}
	if len(got.JSONSchema) == 0 {
		t.Error("the judge sent no schema")
	}
	// Its own limit, not the pipeline's: the pipeline's went from 4096 to
	// 16000, and the instrument must not move with it.
	if got.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want the judge's own 4096", got.MaxTokens)
	}
}

// TestTheJudgeSchemaIgnoresThePipelineFlag: KEYLINT_PYRAMIDIZE_SCHEMA describes
// the pipeline under test. If it reached the judge, a --schema run and a plain
// run would be scored by differently constrained instruments — and comparing
// those two configurations is the only reason the flag exists.
func TestTheJudgeSchemaIgnoresThePipelineFlag(t *testing.T) {
	// enforcedSchema is what the pipeline steps go through; the judge does not.
	if enforcedSchema(judgeSchema) != nil && !schemaEnforcement {
		t.Fatal("test assumption broken: enforcement is off but enforcedSchema returned a schema")
	}

	svc, rec := newTestService()
	rec.client.reply = `{"pyramidStructure":0.8,"clarity":0.8,"completeness":0.8,"tonePreservation":0.8,"overall":0.8,"rationale":"ok"}`
	settingsSvc := settings.NewServiceFrom(settings.Default(), func(string) string { return "key" })

	if _, err := svc.runJudge(settingsSvc, JudgeConfigFromEnv(), "raw", "baseline", "candidate"); err != nil {
		t.Fatalf("RunJudge: %v", err)
	}
	if len(rec.client.gotRequest.JSONSchema) == 0 {
		t.Error("the judge's schema did not survive the pipeline's enforcement flag being off")
	}
}

// TestTheJudgeReadsTheThreeTextsInAFixedOrder: reordering them between runs
// would move the scores for a reason that has nothing to do with the pipeline.
func TestTheJudgeReadsTheThreeTextsInAFixedOrder(t *testing.T) {
	svc, rec := newTestService()
	rec.client.reply = `{"pyramidStructure":0.8,"clarity":0.8,"completeness":0.8,"tonePreservation":0.8,"overall":0.8,"rationale":"ok"}`
	settingsSvc := settings.NewServiceFrom(settings.Default(), func(string) string { return "key" })

	if _, err := svc.runJudge(settingsSvc, JudgeConfigFromEnv(), "RAW", "BASE", "CAND"); err != nil {
		t.Fatalf("RunJudge: %v", err)
	}

	user := rec.client.gotRequest.User
	raw, base, cand := strings.Index(user, "RAW"), strings.Index(user, "BASE"), strings.Index(user, "CAND")
	if raw < 0 || base < 0 || cand < 0 {
		t.Fatalf("the judge did not receive all three texts: %q", user)
	}
	if !(raw < base && base < cand) {
		t.Errorf("order was raw=%d baseline=%d candidate=%d, want that sequence", raw, base, cand)
	}
}

// TestTheCLIJudgeSendsNoTemperature: the default judge must not hand the CLI
// client a pin it silently drops — the request says what the run really did.
func TestTheCLIJudgeSendsNoTemperature(t *testing.T) {
	t.Setenv("EVAL_JUDGE_PROVIDER", "")
	t.Setenv("EVAL_JUDGE_MODEL", "")
	svc, rec := newTestService()
	rec.client.reply = `{"pyramidStructure":0.8,"clarity":0.8,"completeness":0.8,"tonePreservation":0.8,"overall":0.8,"rationale":"ok"}`
	settingsSvc := settings.NewServiceFrom(settings.Default(), nil)

	if _, err := svc.runJudge(settingsSvc, JudgeConfigFromEnv(), "raw", "baseline", "candidate"); err != nil {
		t.Fatalf("runJudge: %v", err)
	}
	if rec.provider != llm.ProviderClaudeCode {
		t.Errorf("provider = %q, want %q", rec.provider, llm.ProviderClaudeCode)
	}
	if rec.client.gotRequest.Temperature != nil {
		t.Errorf("temperature = %v, want none", *rec.client.gotRequest.Temperature)
	}
	if rec.client.gotRequest.Model != defaultJudgeModel {
		t.Errorf("model = %q, want the pinned snapshot", rec.client.gotRequest.Model)
	}
}
