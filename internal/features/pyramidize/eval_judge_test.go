package pyramidize

import (
	"crypto/sha256"
	"encoding/hex"
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

func clearJudgeEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"EVAL_JUDGE_PROVIDER", "EVAL_JUDGE_MODEL", "EVAL_JUDGES"} {
		t.Setenv(k, "")
	}
}

// testJudge is the first judge of the set the environment selects.
func testJudge(t *testing.T) JudgeConfig {
	t.Helper()
	judges, err := JudgesFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	return judges[0]
}

func judgeModels(judges []JudgeConfig) []string {
	models := make([]string, len(judges))
	for i, j := range judges {
		models[i] = j.Model
	}
	return models
}

// TestTwoJudgesByDefault: Haiku 5.5 and Sonnet 5.5 score every sample
// (decision 2026-10-11), both through the CLI, and the pipeline's provider and
// model reach neither — a Sonnet run and an Opus run must go in one table.
func TestTwoJudgesByDefault(t *testing.T) {
	clearJudgeEnv(t)
	t.Setenv("EVAL_PROVIDER", "openai")
	t.Setenv("EVAL_MODEL", "gpt-4.1")

	judges, err := JudgesFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := judgeModels(judges), []string{"claude-haiku-5-5", "claude-sonnet-5-5"}; !reflect.DeepEqual(got, want) {
		t.Errorf("judges = %v, want %v", got, want)
	}
	for _, j := range judges {
		if j.Provider != llm.ProviderClaudeCode {
			t.Errorf("%s: provider = %q, want %q", j.Model, j.Provider, llm.ProviderClaudeCode)
		}
	}
}

// TestTheDefaultJudgesSayTheyAreUnpinned: the CLI has no temperature flag.
// Recording a 0 that never reached the model would claim a repeatability the
// instrument does not have.
func TestTheDefaultJudgesSayTheyAreUnpinned(t *testing.T) {
	clearJudgeEnv(t)
	for _, cfg := range mustJudges(t) {
		if cfg.Temperature != nil {
			t.Errorf("%s: temperature = %v, want nil — the CLI cannot pin it", cfg.Model, *cfg.Temperature)
		}
		if cfg.TemperatureNote == "" {
			t.Errorf("%s: an unpinned temperature carries no note saying why", cfg.Model)
		}
		// It has to reach summary.json as null, not as a missing field or a
		// 0: eval-aggregate.sh keys on it.
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		var back map[string]any
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatal(err)
		}
		if v, ok := back["temperature"]; !ok || v != nil {
			t.Errorf("%s: temperature in JSON = %v (present=%v), want null", cfg.Model, v, ok)
		}
	}
}

func mustJudges(t *testing.T) []JudgeConfig {
	t.Helper()
	judges, err := JudgesFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	return judges
}

// TestAnAPIJudgeIsPinnedOnlyWhereTheModelTakesIt: EVAL_JUDGE_PROVIDER=claude
// is the way back to the API judges. The 5.5 judges reject a non-default
// temperature there, so they record null with a note, like the CLI; the old
// Sonnet 4.5 bridge judge still takes the pin.
func TestAnAPIJudgeIsPinnedOnlyWhereTheModelTakesIt(t *testing.T) {
	clearJudgeEnv(t)
	t.Setenv("EVAL_JUDGE_PROVIDER", llm.ProviderClaude)
	for _, cfg := range mustJudges(t) {
		if cfg.Provider != llm.ProviderClaude {
			t.Errorf("%s: provider = %q, want the API", cfg.Model, cfg.Provider)
		}
		if cfg.Temperature != nil {
			t.Errorf("%s: temperature = %v, want null — the model rejects a pin", cfg.Model, *cfg.Temperature)
		}
		if cfg.TemperatureNote == "" || strings.Contains(cfg.TemperatureNote, "CLI") {
			t.Errorf("%s: note = %q, want one about the model", cfg.Model, cfg.TemperatureNote)
		}
	}

	t.Setenv("EVAL_JUDGES", "claude-sonnet-4-5-20250929")
	cfg := mustJudges(t)[0]
	if cfg.Temperature == nil || *cfg.Temperature != 0 {
		t.Fatalf("bridge judge: temperature = %v, want a pinned 0", cfg.Temperature)
	}
	if cfg.TemperatureNote != "" {
		t.Errorf("bridge judge: a pinned temperature carries a note: %q", cfg.TemperatureNote)
	}
}

// TestTheJudgesAreFullModelIDs: #53 suspected the March baseline had drifted
// under an alias, and an alias is what an instrument must not be. Haiku 5.5
// and Sonnet 5.5 have no dated form; their IDs are the pin, and a run records
// the ID that answered (resolvedModel) so a move would still show.
func TestTheJudgesAreFullModelIDs(t *testing.T) {
	for _, id := range defaultJudgeModels {
		if llm.IsModelFamily(id) || llm.IsClaudeCodeAlias(id) || !strings.HasPrefix(id, "claude-") {
			t.Errorf("judge %q is an alias, not a model ID", id)
		}
	}
}

// TestEvalJudgesChoosesTheSet: a bridge run adds the old Sonnet 4.5 judge as a
// third; order and spacing do not matter to the set, and summary.json records
// what ran.
func TestEvalJudgesChoosesTheSet(t *testing.T) {
	clearJudgeEnv(t)
	t.Setenv("EVAL_JUDGES", " claude-haiku-5-5, claude-sonnet-5-5 ,claude-sonnet-4-5-20250929")
	judges := mustJudges(t)
	if got, want := judgeModels(judges), []string{"claude-haiku-5-5", "claude-sonnet-5-5", "claude-sonnet-4-5-20250929"}; !reflect.DeepEqual(got, want) {
		t.Errorf("judges = %v, want %v", got, want)
	}

	// It has to survive the round trip into summary.json, or a run judged by
	// something else would look like one that was not.
	data, err := json.Marshal(judges)
	if err != nil {
		t.Fatal(err)
	}
	var back []JudgeConfig
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, judges) {
		t.Errorf("round trip = %+v, want %+v", back, judges)
	}
}

// TestEvalJudgeModelIsASetOfOne: the variable from before the judge set still
// works, as a single-judge run — which keys like the single-judge runs before.
func TestEvalJudgeModelIsASetOfOne(t *testing.T) {
	clearJudgeEnv(t)
	t.Setenv("EVAL_JUDGE_PROVIDER", "openai")
	t.Setenv("EVAL_JUDGE_MODEL", "gpt-4.1")
	judges := mustJudges(t)
	if len(judges) != 1 || judges[0].Provider != "openai" || judges[0].Model != "gpt-4.1" {
		t.Errorf("judges = %+v, want openai/gpt-4.1 alone", judges)
	}
}

// TestABadJudgeSetIsRefused: a set with a hole or a duplicate would record
// fewer judges than it claims, and both variables at once leave the reader
// guessing which one ran.
func TestABadJudgeSetIsRefused(t *testing.T) {
	for _, env := range []map[string]string{
		{"EVAL_JUDGES": "claude-haiku-5-5,,claude-sonnet-5-5"},
		{"EVAL_JUDGES": "claude-haiku-5-5,claude-haiku-5-5"},
		{"EVAL_JUDGES": ","},
		{"EVAL_JUDGES": "claude-haiku-5-5", "EVAL_JUDGE_MODEL": "claude-sonnet-5-5"},
	} {
		if judges, err := judgesFrom(envFrom(env)); err == nil {
			t.Errorf("%v was accepted: %+v", env, judges)
		}
	}
}

// TestTheJudgeSendsItsTemperatureAndSchema walks the real call path with a fake
// provider client: the pin is worth nothing if it stops at the struct.
func TestTheJudgeSendsItsTemperatureAndSchema(t *testing.T) {
	// The API judge: the one provider path where a temperature can be pinned.
	clearJudgeEnv(t)
	t.Setenv("EVAL_JUDGE_PROVIDER", llm.ProviderClaude)
	// The bridge judge: the 5.5 judges take no temperature on the API.
	t.Setenv("EVAL_JUDGES", "claude-sonnet-4-5-20250929")
	svc, rec := newTestService()
	judge := testJudge(t)
	rec.client.reply = `{"pyramidStructure":0.8,"clarity":0.8,"completeness":0.8,"tonePreservation":0.8,"overall":0.8,"rationale":"ok"}`
	settingsSvc := settings.NewServiceFrom(settings.Default(), func(string) string { return "key" })

	// Through RunJudge itself, not a hand-built aiOpts: the pin is worth
	// nothing if RunJudge stops applying it.
	if _, err := svc.runJudge(settingsSvc, judge, "raw", "baseline", "candidate"); err != nil {
		t.Fatalf("RunJudge: %v", err)
	}

	got := rec.client.gotRequest
	if got.Temperature == nil {
		t.Fatal("no temperature reached the provider")
	}
	if *got.Temperature != 0 {
		t.Errorf("temperature = %v, want 0", *got.Temperature)
	}
	if got.Model != judge.Model {
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
	judge := testJudge(t)
	rec.client.reply = `{"pyramidStructure":0.8,"clarity":0.8,"completeness":0.8,"tonePreservation":0.8,"overall":0.8,"rationale":"ok"}`
	settingsSvc := settings.NewServiceFrom(settings.Default(), func(string) string { return "key" })

	if _, err := svc.runJudge(settingsSvc, judge, "raw", "baseline", "candidate"); err != nil {
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
	judge := testJudge(t)
	rec.client.reply = `{"pyramidStructure":0.8,"clarity":0.8,"completeness":0.8,"tonePreservation":0.8,"overall":0.8,"rationale":"ok"}`
	settingsSvc := settings.NewServiceFrom(settings.Default(), func(string) string { return "key" })

	if _, err := svc.runJudge(settingsSvc, judge, "RAW", "BASE", "CAND"); err != nil {
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

// TestTheJudgeNeverTakesTheUsersEffort: the judge is a pinned instrument. An
// effort chosen for Pyramidize on the judge's provider must not reach it.
func TestTheJudgeNeverTakesTheUsersEffort(t *testing.T) {
	svc, rec := newTestService()
	judge := testJudge(t)
	rec.client.reply = `{"pyramidStructure":0.8,"clarity":0.8,"completeness":0.8,"tonePreservation":0.8,"overall":0.8,"rationale":"ok"}`
	cfg := settings.Default()
	cfg.Models = map[string]settings.FeatureModels{"claude": {PyramidizeEffort: "max"}}
	settingsSvc := settings.NewServiceFrom(cfg, func(string) string { return "key" })

	if _, err := svc.runJudge(settingsSvc, judge, "raw", "baseline", "candidate"); err != nil {
		t.Fatalf("RunJudge: %v", err)
	}
	if got := rec.client.gotRequest.Effort; got != "" {
		t.Errorf("judge effort = %q, want none", got)
	}
}

// TestTheCLIJudgeSendsNoTemperature: the default judge must not hand the CLI
// client a pin it silently drops — the request says what the run really did.
func TestTheCLIJudgeSendsNoTemperature(t *testing.T) {
	clearJudgeEnv(t)
	svc, rec := newTestService()
	judge := testJudge(t)
	rec.client.reply = `{"pyramidStructure":0.8,"clarity":0.8,"completeness":0.8,"tonePreservation":0.8,"overall":0.8,"rationale":"ok"}`
	settingsSvc := settings.NewServiceFrom(settings.Default(), nil)

	if _, err := svc.runJudge(settingsSvc, judge, "raw", "baseline", "candidate"); err != nil {
		t.Fatalf("runJudge: %v", err)
	}
	if rec.provider != llm.ProviderClaudeCode {
		t.Errorf("provider = %q, want %q", rec.provider, llm.ProviderClaudeCode)
	}
	if rec.client.gotRequest.Temperature != nil {
		t.Errorf("temperature = %v, want none", *rec.client.gotRequest.Temperature)
	}
	if rec.client.gotRequest.Model != judge.Model {
		t.Errorf("model = %q, want the pinned snapshot", rec.client.gotRequest.Model)
	}
}

// TestTheJudgeTreatsMailHistoryAsContext: since 2026-10-11 the output never
// reproduces the mail history in the input (ADR-003, addendum). A judge that
// still counted the history toward completeness would score the intended reply
// down — the v3 run of reply-to-feedback was scored down for the opposite.
func TestTheJudgeTreatsMailHistoryAsContext(t *testing.T) {
	for _, rule := range []string{
		"Mail history in the RAW INPUT",
		"is context",
		"must not reproduce it",
		"Leaving the history out is not a completeness flaw",
		"the author's own message",
	} {
		if !strings.Contains(judgeSystemPrompt, rule) {
			t.Errorf("the judge prompt lacks %q", rule)
		}
	}
	if strings.Contains(judgeSystemPrompt, "Does it preserve ALL information from the raw input?") {
		t.Error("completeness still asks for everything in the raw input, history included")
	}
}

// TestTheRecordedJudgePromptHashIsWhatWasSent: summary.json keys the judge
// prompt by hash (scripts/eval-aggregate.sh), so a change to it reads "not
// comparable". The hash has to describe the system prompt the judge actually
// received, not a copy of it.
func TestTheRecordedJudgePromptHashIsWhatWasSent(t *testing.T) {
	svc, rec := newTestService()
	rec.client.reply = `{"pyramidStructure":0.8,"clarity":0.8,"completeness":0.8,"tonePreservation":0.8,"overall":0.8,"rationale":"ok"}`
	settingsSvc := settings.NewServiceFrom(settings.Default(), func(string) string { return "key" })
	if _, err := svc.runJudge(settingsSvc, testJudge(t), "raw", "baseline", "candidate"); err != nil {
		t.Fatalf("runJudge: %v", err)
	}
	sum := sha256.Sum256([]byte(rec.client.gotRequest.System))
	want := "j1-" + hex.EncodeToString(sum[:8])
	if got := judgePromptHash(); got != want {
		t.Errorf("judgePromptHash() = %q, the judge received a prompt hashing to %q", got, want)
	}
	if instrumentSummary()["judgePromptHash"] != want {
		t.Errorf("summary.json records judgePromptHash %v, want %q", instrumentSummary()["judgePromptHash"], want)
	}
}
