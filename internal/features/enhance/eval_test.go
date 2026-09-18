//go:build eval

package enhance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

// Evaluation for the silent Fix prompt. Real API calls; never part of a normal
// `go test` run.
//
//	go test -tags eval ./internal/features/enhance/ -v -timeout 900s
//	./scripts/eval.sh --suite fix --runs 3
//
// The harness mirrors the Pyramidize one deliberately: same isolated settings,
// same pinned judge, same summary.json shape, so scripts/eval-aggregate.sh works
// on both without knowing which suite produced a run.

// defaultEvalProvider is a constant rather than the machine's active provider:
// a baseline that changes with whoever runs it is not a baseline.
const defaultEvalProvider = llm.ProviderClaude

// loadEvalEnv brings .env in for credentials and lets the command line win for
// everything else. See the Pyramidize eval for why the reverse bit once.
func loadEvalEnv(t *testing.T) {
	t.Helper()
	values, err := godotenv.Read(filepath.Join("..", "..", "..", ".env"))
	if err != nil {
		t.Logf("no .env loaded: %v", err)
		return
	}
	for key, value := range values {
		if strings.HasSuffix(key, "_API_KEY") || os.Getenv(key) == "" {
			t.Setenv(key, value)
		}
	}
}

func gitSHA() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	sha := strings.TrimSpace(string(out))
	if dirty, err := exec.Command("git", "status", "--porcelain", "--untracked-files=no").Output(); err == nil &&
		len(strings.TrimSpace(string(dirty))) > 0 {
		sha += "-dirty"
	}
	return sha
}

// promptHash identifies the prompt a run measured. The git SHA says which commit
// produced it; this says whether the prompt itself moved, which is the thing an
// eval of a prompt is actually comparing.
func promptHash() string {
	// The wrapper is part of the prompt: the markers are what tell the model the
	// text is a document rather than a message, and a run that changed them
	// measured something else. Hashing only the system prompt would have called
	// that the same configuration.
	// The prefix names the formula. It gained the wrapper in this round — the
	// markers are what tell the model the text is a document, so a run that
	// changed them measured something else — and without a version the same
	// prompt would simply hash differently, which reads as "the prompt moved"
	// when only the recipe did.
	sum := sha256.Sum256([]byte(systemPrompt + "\n" + buildUserMessage("")))
	return "p2-" + hex.EncodeToString(sum[:8])
}

type fixSample struct {
	Name      string
	Input     string
	Reference string
	Notes     SampleNotes
}

func loadFixSamples(t *testing.T, split Split) []fixSample {
	t.Helper()
	root := filepath.Join("..", "..", "..", "test-data", "fix-samples")
	refs, err := FixSampleDirs(root)
	if err != nil {
		t.Fatalf("reading samples: %v", err)
	}

	var samples []fixSample
	for _, ref := range refs {
		if !split.Selects(ref.Split) {
			continue
		}
		input, err := os.ReadFile(filepath.Join(ref.Dir, "input.md"))
		if err != nil {
			t.Fatalf("%s: %v", ref.Name, err)
		}
		reference, err := os.ReadFile(filepath.Join(ref.Dir, "reference.md"))
		if err != nil {
			t.Fatalf("%s: %v", ref.Name, err)
		}
		notes, err := parseNotes(filepath.Join(ref.Dir, "notes.md"))
		if err != nil {
			t.Fatalf("%s: %v", ref.Name, err)
		}
		samples = append(samples, fixSample{
			Name:      ref.Name,
			Input:     strings.TrimSpace(string(input)),
			Reference: strings.TrimSpace(string(reference)),
			Notes:     notes,
		})
	}
	if len(samples) == 0 {
		t.Fatalf("no fix samples found for split %q", split)
	}
	return samples
}

func TestEvalFix(t *testing.T) {
	loadEvalEnv(t)

	provider := os.Getenv("EVAL_PROVIDER")
	if provider == "" {
		provider = defaultEvalProvider
	}
	model := os.Getenv("EVAL_MODEL")
	if model == "" {
		model = llm.DefaultModel(provider, llm.FeatureFix)
	}
	judge := JudgeConfigFromEnv(os.Getenv)

	// Explicit configuration and environment-only keys: the developer's own
	// settings file and keyring cannot move a number here.
	evalSettings := settings.Default()
	evalSettings.ActiveProvider = provider
	evalSettings.Models = map[string]settings.FeatureModels{provider: {Fix: model}}
	settingsSvc := settings.NewServiceFrom(evalSettings, settings.EnvOnlyKeys)

	if settingsSvc.GetKey(provider) == "" && provider != llm.ProviderOllama && provider != llm.ProviderClaudeCode {
		t.Fatalf("no API key for %q in the environment or .env — the eval reads no keyring", provider)
	}

	svc := NewService(settingsSvc)
	split, err := SplitFromEnv(os.Getenv("EVAL_SPLIT"))
	if err != nil {
		t.Fatal(err)
	}
	samples := loadFixSamples(t, split)

	timestamp := time.Now().Format("2006-01-02T15-04-05")
	runDir := filepath.Join("..", "..", "..", "test-data", "eval-runs", timestamp)
	outputsDir := filepath.Join(runDir, "samples")
	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		t.Fatalf("creating eval-run dir: %v", err)
	}

	type sampleResult struct {
		Name          string      `json:"name"`
		Deterministic Scorecard   `json:"deterministic"`
		Judge         *JudgeScore `json:"judge,omitempty"`
		Error         string      `json:"error,omitempty"`
		JudgeError    string      `json:"judgeError,omitempty"`
	}

	resultsFile, err := os.Create(filepath.Join(runDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("creating results file: %v", err)
	}
	defer resultsFile.Close()

	totalDet, totalJudge, judgeCount, scored := 0.0, 0.0, 0, 0

	for _, sample := range samples {
		t.Run(sample.Name, func(t *testing.T) {
			sr := sampleResult{Name: sample.Name}

			output, err := svc.Enhance(sample.Input)
			if err != nil {
				sr.Error = err.Error()
				t.Errorf("fix failed: %v", err)
			} else {
				output = strings.TrimSpace(output)
				_ = os.WriteFile(filepath.Join(outputsDir, sample.Name+".md"), []byte(output), 0o644)

				sr.Deterministic = RunDeterministicChecks(sample.Input, sample.Reference, output, sample.Notes)
				totalDet += sr.Deterministic.OverallScore
				scored++

				t.Logf("deterministic: %.2f (pass=%v)", sr.Deterministic.OverallScore, sr.Deterministic.AllPassed)
				for _, c := range sr.Deterministic.Checks {
					t.Logf("  %s: %.2f pass=%v — %s", c.Name, c.Score, c.Pass, c.Detail)
				}

				score, jErr := RunJudge(settingsSvc, judge, sample.Input, sample.Reference, output)
				if jErr != nil {
					// Recorded, not just logged: a judge that dropped an answer
					// leaves judgeCount below sampleCount, and the run folder
					// has to say why once the log is gone.
					sr.JudgeError = jErr.Error()
					t.Logf("judge failed: %v", jErr)
				} else {
					sr.Judge = &score
					totalJudge += score.Overall
					judgeCount++
					t.Logf("judge: overall=%.2f correctness=%.2f meaning=%.2f tone=%.2f noOverEdit=%.2f",
						score.Overall, score.Correctness, score.MeaningPreserved,
						score.TonePreserved, score.NoOverEditing)
					t.Logf("judge rationale: %s", score.Rationale)
				}
			}

			line, _ := json.Marshal(sr)
			fmt.Fprintf(resultsFile, "%s\n", line)
		})
	}

	summary := map[string]any{
		"suite":         "fix",
		"timestamp":     timestamp,
		"gitSHA":        gitSHA(),
		"promptHash":    promptHash(),
		"checksVersion": ChecksVersion,
		// Which half was measured. A tune-half number and an all-samples number
		// are different measurements, and the configKey keeps --compare from
		// mixing them.
		"split": string(split),
		// Which samples, not just how many. The configKey records the count, so
		// two five-sample holdouts with one swapped would compare cleanly; this
		// is the record that says they were not the same five. Enforcement is a
		// test against SPLIT.json, which fails before anything is measured.
		"splitHash":        SplitHash(sampleNames(samples)),
		"provider":         provider,
		"model":            model,
		"judge":            judge,
		"promptVariant":    0, // the Fix prompt has no variants
		"qualityThreshold": 0,
		// Kept for shape-compatibility with the Pyramidize summaries, which is
		// what lets scripts/eval-aggregate.sh read both.
		"schemaEnforcement": false,
		"sampleCount":       len(samples),
		// Divided by what was actually scored, not by what was attempted. A
		// sample whose API call failed used to be averaged in as a zero, which
		// is a measurement of the network rather than of the prompt.
		"avgDeterministic": totalDet / float64(max(scored, 1)),
		"scoredCount":      scored,
		"errorCount":       len(samples) - scored,
	}
	if judgeCount > 0 {
		summary["avgJudge"] = totalJudge / float64(judgeCount)
		summary["judgeCount"] = judgeCount
	}
	data, _ := json.MarshalIndent(summary, "", "  ")
	_ = os.WriteFile(filepath.Join(runDir, "summary.json"), data, 0o644)

	t.Logf("\n=== FIX EVAL SUMMARY ===")
	t.Logf("Provider: %s | Model: %s | Prompt: %s", provider, model, promptHash())
	t.Logf("Judge: %s / %s @ temp %.1f", judge.Provider, judge.Model, judge.Temperature)
	t.Logf("Samples: %d (split %s, membership %s)", len(samples), split, SplitHash(sampleNames(samples)))
	t.Logf("Avg deterministic: %.2f (%d of %d samples scored)", totalDet/float64(max(scored, 1)), scored, len(samples))
	if judgeCount > 0 {
		t.Logf("Avg judge overall: %.2f (%d samples)", totalJudge/float64(judgeCount), judgeCount)
	}
	t.Logf("Results: %s", runDir)
}

// sampleNames is what SplitHash fingerprints.
func sampleNames(samples []fixSample) []string {
	names := make([]string, len(samples))
	for i, s := range samples {
		names[i] = s.Name
	}
	return names
}
