//go:build eval

package pyramidize

// Evaluation tests — make real AI calls against test-data samples.
// Run with: go test -tags eval ./internal/features/pyramidize/ -v -timeout 300s
//
// Requires:
//   - A configured AI provider (env vars: ANTHROPIC_API_KEY or OPENAI_API_KEY)
//   - Network access to the AI provider's API
//
// Override provider/model:
//   EVAL_PROVIDER=claude EVAL_MODEL=claude-sonnet-4-6 go test -tags eval ...

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

// loadEvalEnv brings .env into the environment, letting it win for credentials
// and lose for everything else.
//
// A blanket Overload looked right — a stale ANTHROPIC_API_KEY in the shell
// should not beat the project's own — but it also overwrote the EVAL_* and
// KEYLINT_* variables that scripts/eval.sh exports from its command line, so
// `--model X` silently lost to a line in a file. A run is configured by what the
// caller asked for; .env is a place to keep secrets, not a second opinion on
// what to measure.
func loadEvalEnv(t *testing.T) {
	t.Helper()
	values, err := godotenv.Read(filepath.Join("..", "..", "..", ".env"))
	if err != nil {
		t.Logf("no .env loaded: %v", err)
		return
	}
	for key, value := range values {
		if strings.HasSuffix(key, "_API_KEY") {
			t.Setenv(key, value)
			continue
		}
		if os.Getenv(key) == "" {
			t.Setenv(key, value)
		}
	}
}

// defaultEvalProvider is what a run measures when nothing says otherwise. A
// constant rather than the machine's active provider: a baseline that changes
// with whoever runs it is not a baseline.
const defaultEvalProvider = llm.ProviderClaude

// gitSHA records which commit produced a run, so a number in quality-status.md
// can be traced back to code. Empty when this is not a checkout.
func gitSHA() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	sha := strings.TrimSpace(string(out))
	if dirty, err := exec.Command("git", "status", "--porcelain").Output(); err == nil && len(strings.TrimSpace(string(dirty))) > 0 {
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
	provider := os.Getenv("EVAL_PROVIDER")
	if provider == "" {
		provider = defaultEvalProvider
	}
	// Resolved here and passed as an explicit override, so a run does not depend
	// on whichever model the developer happens to have picked in the GUI — and
	// so the value recorded in summary.json is the one that actually ran.
	model := os.Getenv("EVAL_MODEL")
	if model == "" {
		model = llm.DefaultModel(provider, llm.FeaturePyramidize)
	}
	variant := 0 // latest
	if v := os.Getenv("EVAL_VARIANT"); v != "" {
		fmt.Sscanf(v, "%d", &variant)
	}
	judge := JudgeConfigFromEnv()

	evalSettings := settings.Default()
	evalSettings.ActiveProvider = provider
	settingsSvc := settings.NewServiceFrom(evalSettings, settings.EnvOnlyKeys)

	if settingsSvc.GetKey(provider) == "" && provider != llm.ProviderOllama && provider != llm.ProviderClaudeCode {
		t.Fatalf("no API key for %q in the environment or .env — the eval reads no keyring", provider)
	}

	svc := NewService(settingsSvc, nil)
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

			sr := sampleResult{Name: sample.Name}

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
					score, err := svc.RunJudge(settingsSvc, judge,
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
	summary := map[string]any{
		"timestamp":        timestamp,
		"gitSHA":           gitSHA(),
		"provider":         provider,
		"model":            model,
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
	t.Logf("Provider: %s | Model: %s | Prompt Variant: v%d", provider, model, effectiveVariant)
	t.Logf("Judge: %s / %s @ temp %.1f", judge.Provider, judge.Model, judge.Temperature)
	t.Logf("Schema enforcement: %v | Quality threshold: %.2f", schemaEnforcement, settings.DefaultQualityThreshold)
	t.Logf("Samples: %d", len(samples))
	t.Logf("Avg deterministic: %.2f", totalDet/float64(len(samples)))
	if judgeCount > 0 {
		t.Logf("Avg judge overall: %.2f (%d samples)", totalJudge/float64(judgeCount), judgeCount)
	}
	t.Logf("Results: %s", runDir)
}
