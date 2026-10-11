//go:build eval

package enhance

import (
	"context"
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

// Evaluation for the silent Fix prompt. Real model calls; never part of a normal
// `go test` run. By default pipeline and judge both go through the installed
// Claude Code CLI and no API key is read; EVAL_PROVIDER=claude (and
// EVAL_JUDGE_PROVIDER=claude) measure against the API with ANTHROPIC_API_KEY.
//
//	go test -tags eval ./internal/features/enhance/ -v -timeout 3600s
//	./scripts/eval.sh --suite fix --runs 3
//
// The harness mirrors the Pyramidize one deliberately: same isolated settings,
// same pinned judge, same summary.json shape, so scripts/eval-aggregate.sh works
// on both without knowing which suite produced a run.

// loadEvalEnv brings .env in. What it takes, and why a CLI-only run takes no
// key, is evalEnvFromFile's business.
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

	provider, model := evalTarget(os.Getenv)
	// An alias is resolved once, up front, and the run pins what it resolved
	// to: every sample then runs on the same model, and summary.json records
	// an ID rather than "haiku" — the configKey names the model, so a new
	// generation reads as "not comparable" instead of as a prompt effect.
	requestedModel := model
	model = llm.ResolveModel(context.Background(), provider, model,
		llm.Config{APIKey: settings.EnvOnlyKeys(provider), Feature: "eval"})
	judges, err := JudgesFromEnv(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	judgeProvider := judges[0].Provider // shared by every judge

	// Explicit configuration and environment-only keys: the developer's own
	// settings file and keyring cannot move a number here.
	evalSettings := settings.Default()
	evalSettings.ActiveProvider = provider
	evalSettings.Models = map[string]settings.FeatureModels{provider: {Fix: model}}
	settingsSvc := settings.NewServiceFrom(evalSettings, settings.EnvOnlyKeys)

	for _, p := range []string{provider, judgeProvider} {
		if llm.UsesAPIKey(p) && settingsSvc.GetKey(p) == "" {
			t.Fatalf("no API key for %q in the environment or .env — the eval reads no keyring", p)
		}
	}

	thinkingLabel := fixThinkingLabel(provider, model)
	// Read at the start and again at the end: the CLI is part of the
	// instrument, and an auto-update mid-run would otherwise go unrecorded.
	cliVersionAtStart := evalCLIVersion(provider, judgeProvider)
	svc := NewService(settingsSvc)
	// The Claude Code CLI resolves its aliases itself, so what ran is read
	// off the responses. Each judge has a recorder of its own, so no judge's
	// model is counted as the pipeline's or as another judge's.
	var answered llm.ModelRecorder
	svc.newClient = answered.Wrap(svc.newClient)
	judgeAnswered := make([]llm.ModelRecorder, len(judges))
	judgeClients := make([]func(string, llm.Config) (llm.Client, error), len(judges))
	for i := range judges {
		judgeClients[i] = judgeAnswered[i].Wrap(llm.New)
	}
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
		Name          string    `json:"name"`
		Deterministic Scorecard `json:"deterministic"`
		// Each judge's score, keyed by its model; never averaged.
		Judges      map[string]*JudgeScore `json:"judges,omitempty"`
		Error       string                 `json:"error,omitempty"`
		JudgeErrors map[string]string      `json:"judgeErrors,omitempty"`
	}

	resultsFile, err := os.Create(filepath.Join(runDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("creating results file: %v", err)
	}
	defer resultsFile.Close()

	totalDet, scored := 0.0, 0
	judgeTotal := map[string]float64{}
	judgeCount := map[string]int{}
	for _, j := range judges {
		judgeCount[j.Model] = 0
	}

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

				for i, judge := range judges {
					score, jErr := RunJudge(settingsSvc, judge, judgeClients[i], sample.Input, sample.Reference, output)
					if jErr != nil {
						// Recorded, not just logged: a judge that dropped an
						// answer leaves its judgeCount below sampleCount, and
						// the run folder has to say why once the log is gone.
						if sr.JudgeErrors == nil {
							sr.JudgeErrors = map[string]string{}
						}
						sr.JudgeErrors[judge.Model] = jErr.Error()
						t.Logf("judge %s failed: %v", judge.Model, jErr)
						continue
					}
					if sr.Judges == nil {
						sr.Judges = map[string]*JudgeScore{}
					}
					sr.Judges[judge.Model] = &score
					judgeTotal[judge.Model] += score.Overall
					judgeCount[judge.Model]++
					t.Logf("judge %s: overall=%.2f correctness=%.2f meaning=%.2f tone=%.2f noOverEdit=%.2f",
						judge.Model, score.Overall, score.Correctness, score.MeaningPreserved,
						score.TonePreserved, score.NoOverEditing)
					t.Logf("judge %s rationale: %s", judge.Model, score.Rationale)
				}
			}

			line, _ := json.Marshal(sr)
			fmt.Fprintf(resultsFile, "%s\n", line)
		})
	}

	avgJudges := map[string]float64{}
	for i := range judges {
		judges[i].ResolvedModel = judgeAnswered[i].Recorded(judges[i].Model)
		if n := judgeCount[judges[i].Model]; n > 0 {
			avgJudges[judges[i].Model] = judgeTotal[judges[i].Model] / float64(n)
		}
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
		"splitHash": SplitHash(sampleNames(samples)),
		"provider":  provider,
		// What answered, never the alias; see ModelRecorder.
		"model":          answered.Recorded(model),
		"modelRequested": requestedModel,
		// The CLI is part of the instrument when either side runs through it:
		// its defaults sit between the model and the score. In the configKey.
		"claudeCodeVersion": cliVersionSpan(cliVersionAtStart, evalCLIVersion(provider, judgeProvider)),
		// What Fix does about reasoning on this route; see fixThinkingLabel.
		"thinking": thinkingLabel,
		// Every judge, with the ID that answered for it; their scores stay
		// apart in avgJudges and judgeCounts, keyed by judge model.
		"judges":           judges,
		"avgJudges":        avgJudges,
		"judgeCounts":      judgeCount,
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
	data, _ := json.MarshalIndent(summary, "", "  ")
	_ = os.WriteFile(filepath.Join(runDir, "summary.json"), data, 0o644)

	t.Logf("\n=== FIX EVAL SUMMARY ===")
	t.Logf("Provider: %s | Model: %s (resolved: %s) | Thinking: %s | Prompt: %s", provider, requestedModel, answered.Recorded(model), thinkingLabel, promptHash())
	for _, j := range judges {
		t.Logf("Judge: %s / %s (resolved: %s) @ temp %s", j.Provider, j.Model, j.ResolvedModel, j.TemperatureLabel())
	}
	t.Logf("Samples: %d (split %s, membership %s)", len(samples), split, SplitHash(sampleNames(samples)))
	t.Logf("Avg deterministic: %.2f (%d of %d samples scored)", totalDet/float64(max(scored, 1)), scored, len(samples))
	for _, j := range judges {
		if n := judgeCount[j.Model]; n > 0 {
			t.Logf("Avg judge %s overall: %.2f (%d samples)", j.Model, avgJudges[j.Model], n)
		}
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
