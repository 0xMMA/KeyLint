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
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"keylint/internal/features/settings"
)

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

func parseTestData(content string) (rawInput, baseline string) {
	sections := strings.Split(content, "# User accepted output")
	if len(sections) < 2 {
		return "", ""
	}

	rawSection := strings.TrimPrefix(sections[0], "# Raw Input")
	if m := fenceBlockRegex.FindStringSubmatch(rawSection); len(m) > 1 {
		rawInput = strings.TrimSpace(m[1])
	}
	if m := fenceBlockRegex.FindStringSubmatch(sections[1]); len(m) > 1 {
		baseline = strings.TrimSpace(m[1])
	}
	return
}

func TestEvalPyramidize(t *testing.T) {
	settingsSvc, err := settings.NewService()
	if err != nil {
		t.Fatalf("settings init: %v", err)
	}

	provider := os.Getenv("EVAL_PROVIDER")
	model := os.Getenv("EVAL_MODEL")

	svc := NewService(settingsSvc, nil)
	samples := loadTestSamples(t)

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
					judge, err := RunJudge(settingsSvc, aiOpts{provider: provider, model: model},
						sample.RawInput, sample.Baseline, result.FullDocument)
					if err != nil {
						t.Logf("judge failed: %v", err)
					} else {
						sr.Judge = &judge
						totalJudge += judge.Overall
						judgeCount++
						t.Logf("judge: overall=%.2f pyramid=%.2f clarity=%.2f completeness=%.2f tone=%.2f",
							judge.Overall, judge.PyramidStructure, judge.Clarity, judge.Completeness, judge.TonePreservation)
						t.Logf("judge rationale: %s", judge.Rationale)
					}
				}
			}

			// Write result line.
			line, _ := json.Marshal(sr)
			fmt.Fprintf(resultsFile, "%s\n", line)
		})
	}

	// Write summary.
	summary := map[string]any{
		"timestamp":        timestamp,
		"provider":         provider,
		"model":            model,
		"sampleCount":      len(samples),
		"avgDeterministic": totalDet / float64(len(samples)),
	}
	if judgeCount > 0 {
		summary["avgJudge"] = totalJudge / float64(judgeCount)
		summary["judgeCount"] = judgeCount
	}
	summaryData, _ := json.MarshalIndent(summary, "", "  ")
	os.WriteFile(filepath.Join(runDir, "summary.json"), summaryData, 0644)

	t.Logf("\n=== EVAL SUMMARY ===")
	t.Logf("Samples: %d", len(samples))
	t.Logf("Avg deterministic: %.2f", totalDet/float64(len(samples)))
	if judgeCount > 0 {
		t.Logf("Avg judge overall: %.2f (%d samples)", totalJudge/float64(judgeCount), judgeCount)
	}
	t.Logf("Results: %s", runDir)
}
