//go:build eval

package pyramidize

// Evaluation tests — make real AI calls against test-data samples.
// Run with: go test -tags eval ./internal/features/pyramidize/ -v -timeout 10800s
//
// Requires, by default, nothing but a signed-in Claude Code CLI: pipeline and
// judge both run through it (provider claude-code), so no API key is read.
//
// Against the API instead (needs ANTHROPIC_API_KEY in .env or the environment):
//   EVAL_PROVIDER=claude EVAL_MODEL=claude-sonnet-4-6 go test -tags eval ...
//   EVAL_JUDGE_PROVIDER=claude moves the judge back to the API as well.

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"keylint/internal/features/settings"
	"keylint/internal/llm"
)

// loadEvalEnv brings .env into the environment. What it takes, and why a
// CLI-only run takes no key, is evalEnvFromFile's business.
func loadEvalEnv(t *testing.T) {
	t.Helper()
	values, err := godotenv.Read(filepath.Join("..", "..", "..", ".env"))
	if err != nil {
		t.Logf("no .env loaded: %v", err)
		return
	}
	for key, value := range evalEnvFromFile(values, os.Getenv) {
		t.Setenv(key, value)
	}
}

// gitSHA records which commit produced a run, so a number in quality-status.md
// can be traced back to code. Empty when this is not a checkout.
func gitSHA() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	sha := strings.TrimSpace(string(out))
	// Tracked changes only. A run writes into test-data/eval-runs/ and
	// test-data/eval-baselines/, so counting untracked files would mark every
	// run after the first as dirty and make the marker meaningless.
	if dirty, err := exec.Command("git", "status", "--porcelain", "--untracked-files=no").Output(); err == nil && len(strings.TrimSpace(string(dirty))) > 0 {
		sha += "-dirty"
	}
	return sha
}

// loadTestSamples parses test-data/pyramidal-emails (format and history
// markers: eval_samples.go). A malformed sample stops the run before any call
// is paid for.
func loadTestSamples(t *testing.T) []evalSample {
	t.Helper()
	samples, skipped, err := loadEvalSamples(filepath.Join("..", "..", "..", "test-data", "pyramidal-emails"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range skipped {
		t.Logf("skipping %s: no raw input found", name)
	}
	if len(samples) == 0 {
		t.Fatal("no test samples found")
	}
	return samples
}

func TestEvalPyramidize(t *testing.T) {
	loadEvalEnv(t)

	// A measurement must not depend on the machine it runs on. The settings
	// service is built from an explicit configuration and an environment-only
	// key lookup, so neither ~/.config/KeyLint/settings.json nor the OS keyring
	// can change what this run produces — a developer whose GUI has Ollama as
	// the active provider gets the same numbers as anyone else.
	provider, model := evalTarget(os.Getenv)
	variant := 0 // latest
	if v := os.Getenv("EVAL_VARIANT"); v != "" {
		fmt.Sscanf(v, "%d", &variant)
	}
	judges, err := JudgesFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	judgeProvider := judges[0].Provider // shared by every judge

	evalSettings := settings.Default()
	evalSettings.ActiveProvider = provider
	settingsSvc := settings.NewServiceFrom(evalSettings, settings.EnvOnlyKeys)

	// An alias is resolved once, up front, and the run pins what it resolved
	// to; summary.json then records an ID, never the alias. See TestEvalFix.
	requestedModel := model
	model = llm.ResolveModel(context.Background(), provider, model,
		llm.Config{APIKey: settingsSvc.GetKey(provider), Feature: "eval"})

	for _, p := range []string{provider, judgeProvider} {
		if llm.UsesAPIKey(p) && settingsSvc.GetKey(p) == "" {
			t.Fatalf("no API key for %q in the environment or .env — the eval reads no keyring", p)
		}
	}

	// Read at the start and again at the end: the CLI is part of the
	// instrument, and an auto-update mid-run would otherwise go unrecorded.
	cliVersionAtStart := evalCLIVersion(provider, judgeProvider)
	svc := NewService(settingsSvc, nil)
	// What ran is read off the responses, for the Claude Code CLI's aliases.
	// Each judge runs on a service of its own, with a recorder of its own, so
	// no judge's model is counted as the pipeline's or as another judge's.
	var answered llm.ModelRecorder
	svc.newClient = answered.Wrap(svc.newClient)
	judgeSvcs := make([]*Service, len(judges))
	judgeAnswered := make([]llm.ModelRecorder, len(judges))
	for i := range judges {
		judgeSvcs[i] = NewService(settingsSvc, nil)
		judgeSvcs[i].newClient = judgeAnswered[i].Wrap(judgeSvcs[i].newClient)
	}
	// Refused up front: a variant that does not exist for email would
	// otherwise fail every sample one call at a time.
	effectiveVariant, err := resolvePromptVariant("email", variant)
	if err != nil {
		t.Fatal(err)
	}
	promptHash, err := evalPromptHash(variant)
	if err != nil {
		t.Fatal(err)
	}
	samples := loadTestSamples(t)
	t.Logf("prompt variant: %d (0=latest=%d)", variant, LatestPromptVariant)

	// Create eval run directory.
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	runDir := filepath.Join("..", "..", "..", "test-data", "eval-runs", timestamp)
	samplesDir := filepath.Join(runDir, "samples")
	if err := os.MkdirAll(samplesDir, 0755); err != nil {
		t.Fatalf("creating eval-run dir: %v", err)
	}

	type sampleResult struct {
		Name          string        `json:"name"`
		Deterministic EvalScorecard `json:"deterministic"`
		// Judges holds each judge's score, keyed by its model; never averaged.
		Judges map[string]*JudgeScore `json:"judges,omitempty"`
		Error  string                 `json:"error,omitempty"`
		// JudgeErrors is recorded, not just logged: a judge that dropped an
		// answer leaves its judgeCount below scoredCount, and the run folder
		// has to say why once the log is gone.
		JudgeErrors map[string]string `json:"judgeErrors,omitempty"`
		// AppliedRefinement said whether this sample cost a second model call.
		// Since ADR-003 every run is one call (pinned by oneshot_test.go), so it
		// is always false — and still written, so the aggregate reports 0
		// refined rather than "unrecorded" (null).
		AppliedRefinement bool `json:"appliedRefinement"`
	}

	resultsFile, err := os.Create(filepath.Join(runDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("creating results file: %v", err)
	}
	defer resultsFile.Close()

	totalDet := 0.0
	scored := 0
	judgeTotal := map[string]float64{}
	judgeCount := map[string]int{}
	for _, j := range judges {
		judgeCount[j.Model] = 0
	}

	for _, sample := range samples {
		t.Run(sample.Name, func(t *testing.T) {
			result, err := svc.Pyramidize(PyramidizeRequest{
				Text:               sample.RawInput,
				DocumentType:       "email",
				CommunicationStyle: "professional",
				RelationshipLevel:  "professional",
				Provider:           provider,
				Model:              model,
				PromptVariant:      variant,
			})

			sr := sampleResult{Name: sample.Name}

			if err != nil {
				sr.Error = err.Error()
				t.Errorf("pyramidize failed: %v", err)
			} else {
				// Save generated output.
				outPath := filepath.Join(samplesDir, sample.Name+".md")
				os.WriteFile(outPath, []byte(result.FullDocument), 0644)

				// Deterministic checks.
				sr.Deterministic = RunDeterministicChecks(sample.RawInput, sample.AuthorText, result.FullDocument)
				totalDet += sr.Deterministic.OverallScore
				scored++

				t.Logf("deterministic: %.2f (pass=%v)", sr.Deterministic.OverallScore, sr.Deterministic.AllPassed)
				for _, c := range sr.Deterministic.Checks {
					t.Logf("  %s: %.2f pass=%v — %s", c.Name, c.Score, c.Pass, c.Detail)
				}

				// LLM-as-judge (if baseline available), every judge in turn.
				for i, judge := range judges {
					if sample.Baseline == "" {
						break
					}
					score, err := judgeSvcs[i].runJudge(settingsSvc, judge,
						sample.RawInput, sample.Baseline, result.FullDocument)
					if err != nil {
						if sr.JudgeErrors == nil {
							sr.JudgeErrors = map[string]string{}
						}
						sr.JudgeErrors[judge.Model] = err.Error()
						t.Logf("judge %s failed: %v", judge.Model, err)
						continue
					}
					if sr.Judges == nil {
						sr.Judges = map[string]*JudgeScore{}
					}
					sr.Judges[judge.Model] = &score
					judgeTotal[judge.Model] += score.Overall
					judgeCount[judge.Model]++
					t.Logf("judge %s: overall=%.2f pyramid=%.2f clarity=%.2f completeness=%.2f tone=%.2f",
						judge.Model, score.Overall, score.PyramidStructure, score.Clarity, score.Completeness, score.TonePreservation)
					t.Logf("judge %s rationale: %s", judge.Model, score.Rationale)
				}
			}

			// Write result line.
			line, _ := json.Marshal(sr)
			fmt.Fprintf(resultsFile, "%s\n", line)
		})
	}

	// Write summary. Everything a reader needs to reproduce this run goes in
	// here: a number without its configuration is not a measurement, and the
	// March table in quality-status.md became unreadable for exactly that
	// reason — nobody could tell which model had produced it.
	avgJudges := map[string]float64{}
	for i := range judges {
		judges[i].ResolvedModel = judgeAnswered[i].Recorded(judges[i].Model)
		if n := judgeCount[judges[i].Model]; n > 0 {
			avgJudges[judges[i].Model] = judgeTotal[judges[i].Model] / float64(n)
		}
	}
	summary := map[string]any{
		"timestamp": timestamp,
		"gitSHA":    gitSHA(),
		"provider":  provider,
		// What answered, never the alias; see llm.ModelRecorder.
		"model":          answered.Recorded(model),
		"modelRequested": requestedModel,
		// The CLI is part of the instrument when either side runs through it:
		// its defaults sit between the model and the score. Reported next to a
		// verdict, not keyed (the CLI updates itself).
		"claudeCodeVersion": cliVersionSpan(cliVersionAtStart, evalCLIVersion(provider, judgeProvider)),
		"thinking":          pyramidizeThinking(provider),
		"promptVariant":     effectiveVariant,
		// Which prompt, not only which variant: reported, not keyed.
		"promptHash": promptHash,
		// Every judge, with the ID that answered for it. Their scores stay
		// apart: avgJudges and judgeCounts are keyed by judge model.
		"judges":      judges,
		"avgJudges":   avgJudges,
		"judgeCounts": judgeCount,
		// Inert since ADR-003 (no refine pass reads it), still recorded:
		// it is part of the configKey, and dropping it would make every
		// earlier baseline "not comparable" for a field that changed nothing.
		"qualityThreshold": settings.DefaultQualityThreshold,
		// Which configuration produced these numbers; see schemas.go.
		"schemaEnforcement": schemaEnforcement,
		"sampleCount":       len(samples),
		// Divided by what was scored, not by what was attempted: a sample whose
		// call failed is a measurement of the network, not of the prompt. The
		// counts say how many that was; the aggregate refuses to compare a run
		// that did not score every sample.
		"avgDeterministic": totalDet / float64(max(scored, 1)),
		"scoredCount":      scored,
		"errorCount":       len(samples) - scored,
	}
	// The scoring instrument: checksVersion and judgePromptHash, both keyed.
	// A run scored by other checks or a reworded judge is another measurement.
	maps.Copy(summary, instrumentSummary())
	summaryData, _ := json.MarshalIndent(summary, "", "  ")
	os.WriteFile(filepath.Join(runDir, "summary.json"), summaryData, 0644)

	t.Logf("\n=== EVAL SUMMARY ===")
	t.Logf("Provider: %s | Model: %s (resolved: %s) | Prompt Variant: v%d | Prompt: %s", provider, requestedModel, answered.Recorded(model), effectiveVariant, promptHash)
	for _, j := range judges {
		t.Logf("Judge: %s / %s (resolved: %s) @ temp %s", j.Provider, j.Model, j.ResolvedModel, j.TemperatureLabel())
	}
	t.Logf("Schema enforcement: %v | Quality threshold: %.2f | Checks: v%d | Judge prompt: %s", schemaEnforcement, settings.DefaultQualityThreshold, ChecksVersion, judgePromptHash())
	t.Logf("Samples: %d", len(samples))
	t.Logf("Avg deterministic: %.2f (%d of %d samples scored)", totalDet/float64(max(scored, 1)), scored, len(samples))
	for _, j := range judges {
		if n := judgeCount[j.Model]; n > 0 {
			t.Logf("Avg judge %s overall: %.2f (%d samples)", j.Model, avgJudges[j.Model], n)
		}
	}
	t.Logf("Results: %s", runDir)
}
