//go:build eval

package pyramidize

// TestEvalLanguage: is every reply written in the language it should be — the
// author's, or the one the user's instructions ask for? One model call per
// sample and run, no judge; checkLanguage / checkLanguageAs decide.
//
//   go test -tags eval,gtk3 ./internal/features/pyramidize/ -run TestEvalLanguage -v -timeout 3600s
//
// EVAL_RUNS=n repeats each sample n times (default 3): 9 samples × 3 = 27
// calls. EVAL_PROVIDER / EVAL_MODEL as for TestEvalPyramidize. Replies and a
// summary.json go to test-data/eval-runs/<timestamp>-language/. eval.sh does
// not run this test.
//
// Why it exists: on 2026-10-14 English notes came back in German in 6 of 7
// diagnostic calls (v2: 3 of 3), and the main suite could not see it — its
// samples are all German. The samples (languageSamples, eval_language_test.go)
// are English in every form, German email and chat, a German mail with an
// English greeting and sign-off, which stays German (the check does not ask
// whether the greeting stays English: it has no function words to count), and
// a German mail whose instructions ask for English.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
)

func TestEvalLanguage(t *testing.T) {
	loadEvalEnv(t)

	// Isolated as TestEvalPyramidize is: an explicit configuration and
	// environment-only keys, so the GUI's settings cannot move the result.
	provider, requestedModel := evalTarget(os.Getenv)
	evalSettings := settings.Default()
	evalSettings.ActiveProvider = provider
	settingsSvc := settings.NewServiceFrom(evalSettings, settings.EnvOnlyKeys)
	if llm.UsesAPIKey(provider) && settingsSvc.GetKey(provider) == "" {
		t.Fatalf("no API key for %q in the environment or .env — the eval reads no keyring", provider)
	}
	model := llm.ResolveModel(context.Background(), provider, requestedModel,
		llm.Config{APIKey: settingsSvc.GetKey(provider), Feature: "eval"})

	runs := 3
	if v := strings.TrimSpace(os.Getenv("EVAL_RUNS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			t.Fatalf("EVAL_RUNS=%q: want a positive number", v)
		}
		runs = n
	}
	promptHash, err := evalPromptHash(0)
	if err != nil {
		t.Fatal(err)
	}
	cliVersionAtStart := evalCLIVersion(provider)

	runDir := filepath.Join("..", "..", "..", "test-data", "eval-runs",
		time.Now().Format("2006-01-02T15-04-05")+"-language")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}

	svc := NewService(settingsSvc, nil)
	// What answered, read off the responses: the alias is not the model.
	var answered llm.ModelRecorder
	svc.newClient = answered.Wrap(svc.newClient)

	type sampleResult struct {
		Name   string   `json:"name"`
		Want   string   `json:"want"`
		Passed int      `json:"passed"`
		Failed int      `json:"failed"`
		Errors int      `json:"errors"`
		Detail []string `json:"detail"`
	}
	var results []sampleResult
	passed, failed, errored := 0, 0, 0
	for _, s := range languageSamples {
		sr := sampleResult{Name: s.name, Want: s.want}
		t.Run(s.name, func(t *testing.T) {
			for run := 1; run <= runs; run++ {
				result, err := svc.Pyramidize(PyramidizeRequest{
					Text:               s.text,
					DocumentType:       s.form,
					CustomInstructions: s.instructions,
					Provider:           provider,
					Model:              model,
				})
				// A failed call says nothing about language: counted apart.
				if err != nil {
					sr.Errors++
					sr.Detail = append(sr.Detail, fmt.Sprintf("run %d: error: %v", run, err))
					t.Errorf("run %d: %v", run, err)
					continue
				}
				out := filepath.Join(runDir, fmt.Sprintf("%s-%d.md", s.name, run))
				if err := os.WriteFile(out, []byte(result.FullDocument), 0644); err != nil {
					t.Fatal(err)
				}
				c := checkLanguage(s.text, result.FullDocument)
				if s.want != s.lang {
					c = checkLanguageAs(s.want, result.FullDocument)
				}
				sr.Detail = append(sr.Detail, fmt.Sprintf("run %d: %s", run, c.Detail))
				if c.Pass {
					sr.Passed++
					t.Logf("run %d: %s", run, c.Detail)
				} else {
					sr.Failed++
					t.Errorf("run %d: %s — %s", run, c.Detail, out)
				}
			}
		})
		passed += sr.Passed
		failed += sr.Failed
		errored += sr.Errors
		results = append(results, sr)
	}

	summary := map[string]any{
		"suite":             "pyramidize-language",
		"timestamp":         time.Now().Format(time.RFC3339),
		"gitSHA":            gitSHA(),
		"provider":          provider,
		"modelRequested":    requestedModel,
		"model":             answered.Recorded(model),
		"claudeCodeVersion": cliVersionSpan(cliVersionAtStart, evalCLIVersion(provider)),
		"promptHash":        promptHash,
		"runsPerSample":     runs,
		"passed":            passed,
		"failed":            failed,
		"errors":            errored,
		"samples":           results,
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "summary.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d of %d replies in the language they should be (%d calls failed); %s, model %s, prompt %s",
		passed, passed+failed, errored, runDir, answered.Recorded(model), promptHash)
}
