package pyramidize

import (
	"encoding/json"
	"strings"
	"testing"

	"keylint/internal/features/settings"
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
	if cfg.Temperature != 0 {
		t.Errorf("temperature = %v, want 0 — the same three texts should score the same twice", cfg.Temperature)
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
	if back != cfg {
		t.Errorf("round trip = %+v, want %+v", back, cfg)
	}
}

// TestTheJudgeSendsItsTemperatureAndSchema walks the real call path with a fake
// provider client: the pin is worth nothing if it stops at the struct.
func TestTheJudgeSendsItsTemperatureAndSchema(t *testing.T) {
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
	if got.MaxTokens == maxTokens {
		t.Error("the judge sent the pipeline's output limit")
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
