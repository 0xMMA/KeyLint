//go:build eval

package pyramidize

// Evaluation tests — make real AI calls against test-data samples.
// Run with: go test -tags eval ./internal/features/pyramidize/ -v -timeout 900s
//
// Requires, by default, nothing but a signed-in Claude Code CLI: pipeline and
// judge both run through it (provider claude-code), so no API key is read.
//
// Against the API instead (needs ANTHROPIC_API_KEY in .env or the environment):
//   EVAL_PROVIDER=claude EVAL_MODEL=claude-sonnet-4-6 go test -tags eval ...
//   EVAL_JUDGE_PROVIDER=claude moves the judge back to the API as well.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// testSample holds one parsed test-data file.
type testSample struct {
	Name     string
	RawInput string
	Baseline string
}

func loadTestSamples(t *testing.T) []testSample {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "test-data", "pyramidal-emails")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read test-data dir: %v", err)
	}

	var samples []testSample
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		raw, baseline := parseTestData(string(data))
		if raw == "" {
			t.Logf("skipping %s: no raw input found", e.Name())
			continue
		}
		samples = append(samples, testSample{
			Name:     strings.TrimSuffix(e.Name(), ".md"),
			RawInput: raw,
			Baseline: baseline,
		})
	}
	if len(samples) == 0 {
		t.Fatal("no test samples found")
	}
	return samples
}

var fenceBlockRegex = regexp.MustCompile("(?s)```\\w*\\n?(.*?)```")

// rawInputHeader matches "# Raw Input" / "# Raw input" / "# raw input" (case-insensitive).
var rawInputHeader = regexp.MustCompile(`(?im)^#\s+raw\s+input\s*$`)

// baselineHeader matches "# User accepted output(s)" including the typo "accpted" (case-insensitive).
var baselineHeader = regexp.MustCompile(`(?im)^#\s+user\s+acce?pted\s+outputs?\s*$`)

func parseTestData(content string) (rawInput, baseline string) {
	// Find the baseline header position first — it delimits the two sections.
	blLoc := baselineHeader.FindStringIndex(content)
	if blLoc == nil {
		return "", ""
	}

	rawSection := content[:blLoc[0]]
	baselineSection := content[blLoc[1]:]

	// Strip the raw-input header from the first section.
	rawSection = rawInputHeader.ReplaceAllString(rawSection, "")

	// Try fenced block first; fall back to plain text after stripping the header.
	if m := fenceBlockRegex.FindStringSubmatch(rawSection); len(m) > 1 {
		rawInput = strings.TrimSpace(m[1])
	} else {
		rawInput = strings.TrimSpace(rawSection)
	}
	if m := fenceBlockRegex.FindStringSubmatch(baselineSection); len(m) > 1 {
		baseline = strings.TrimSpace(m[1])
	} else {
		baseline = strings.TrimSpace(baselineSection)
	}
	return
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
	judge := JudgeConfigFromEnv()

	evalSettings := settings.Default()
	evalSettings.ActiveProvider = provider
	settingsSvc := settings.NewServiceFrom(evalSettings, settings.EnvOnlyKeys)

	for _, p := range []string{provider, judge.Provider} {
		if llm.UsesAPIKey(p) && settingsSvc.GetKey(p) == "" {
			t.Fatalf("no API key for %q in the environment or .env — the eval reads no keyring", p)
		}
	}

	// What actually answered, for the pipeline and the judge separately. The
	// pipeline asks for an alias, and an alias names a different model the day
	// the provider ships a new generation; summary.json records the ID, and the
	// configKey keys on it, so that day reads "not comparable" rather than
	// looking like a prompt effect. Two services so the two recorders cannot
	// mix: the judge goes through the same callAISync as the pipeline.
	var pipelineModels, judgeModels llm.ResolvedModels
	svc := NewService(settingsSvc, nil)
	svc.newClient = pipelineModels.WrapFactory(llm.New)
	judgeSvc := NewService(settingsSvc, nil)
	judgeSvc.newClient = judgeModels.WrapFactory(llm.New)
	samples := loadTestSamples(t)
	t.Logf("prompt variant: %d (0=latest=%d)", variant, LatestEmailVariant)

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
		Judge         *JudgeScore   `json:"judge,omitempty"`
		Error         string        `json:"error,omitempty"`
		// AppliedRefinement says whether this sample cost a second model call.
		// Without it a run cannot show whether the pipeline arm of a comparison
		// ever behaved like a pipeline — the gap ADR-002 had to record as
		// unfalsifiable from its own data.
		AppliedRefinement bool `json:"appliedRefinement"`
	}

	resultsFile, err := os.Create(filepath.Join(runDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("creating results file: %v", err)
	}
	defer resultsFile.Close()

	totalDet := 0.0
	totalJudge := 0.0
	judgeCount := 0

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

			sr := sampleResult{Name: sample.Name, AppliedRefinement: result.AppliedRefinement}

			if err != nil {
				sr.Error = err.Error()
				t.Errorf("pyramidize failed: %v", err)
			} else {
				// Save generated output.
				outPath := filepath.Join(samplesDir, sample.Name+".md")
				os.WriteFile(outPath, []byte(result.FullDocument), 0644)

				// Deterministic checks.
				sr.Deterministic = RunDeterministicChecks(sample.RawInput, result.FullDocument)
				totalDet += sr.Deterministic.OverallScore

				t.Logf("deterministic: %.2f (pass=%v)", sr.Deterministic.OverallScore, sr.Deterministic.AllPassed)
				for _, c := range sr.Deterministic.Checks {
					t.Logf("  %s: %.2f pass=%v — %s", c.Name, c.Score, c.Pass, c.Detail)
				}

				// LLM-as-judge (if baseline available).
				if sample.Baseline != "" {
					score, err := judgeSvc.runJudge(settingsSvc, judge,
						sample.RawInput, sample.Baseline, result.FullDocument)
					if err != nil {
						t.Logf("judge failed: %v", err)
					} else {
						sr.Judge = &score
						totalJudge += score.Overall
						judgeCount++
						t.Logf("judge: overall=%.2f pyramid=%.2f clarity=%.2f completeness=%.2f tone=%.2f",
							score.Overall, score.PyramidStructure, score.Clarity, score.Completeness, score.TonePreservation)
						t.Logf("judge rationale: %s", score.Rationale)
					}
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
	effectiveVariant := variant
	if effectiveVariant == 0 {
		effectiveVariant = LatestEmailVariant
	}
	judge.ResolvedModel = judgeModels.String()
	summary := map[string]any{
		"timestamp": timestamp,
		"gitSHA":    gitSHA(),
		"provider":  provider,
		"model":     model,
		// The ID that answered, which the configKey uses in place of model.
		// Empty only when no call succeeded, and then the key falls back to the
		// requested model.
		"resolvedModel":    pipelineModels.String(),
		"promptVariant":    effectiveVariant,
		"judge":            judge,
		"qualityThreshold": settings.DefaultQualityThreshold,
		// Which configuration produced these numbers; see schemas.go.
		"schemaEnforcement": schemaEnforcement,
		"sampleCount":       len(samples),
		"avgDeterministic":  totalDet / float64(len(samples)),
	}
	if judgeCount > 0 {
		summary["avgJudge"] = totalJudge / float64(judgeCount)
		summary["judgeCount"] = judgeCount
	}
	summaryData, _ := json.MarshalIndent(summary, "", "  ")
	os.WriteFile(filepath.Join(runDir, "summary.json"), summaryData, 0644)

	t.Logf("\n=== EVAL SUMMARY ===")
	t.Logf("Provider: %s | Model: %s (resolved: %s) | Prompt Variant: v%d", provider, model, pipelineModels.String(), effectiveVariant)
	t.Logf("Judge: %s / %s (resolved: %s) @ temp %s", judge.Provider, judge.Model, judge.ResolvedModel, judge.TemperatureLabel())
	t.Logf("Schema enforcement: %v | Quality threshold: %.2f", schemaEnforcement, settings.DefaultQualityThreshold)
	t.Logf("Samples: %d", len(samples))
	t.Logf("Avg deterministic: %.2f", totalDet/float64(len(samples)))
	if judgeCount > 0 {
		t.Logf("Avg judge overall: %.2f (%d samples)", totalJudge/float64(judgeCount), judgeCount)
	}
	t.Logf("Results: %s", runDir)
}
