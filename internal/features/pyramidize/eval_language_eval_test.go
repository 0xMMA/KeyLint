//go:build eval

package pyramidize

// TestEvalLanguage: is every reply written in the language of the author's
// message? One model call per sample and run, no judge; checkLanguage decides.
//
//   go test -tags eval,gtk3 ./internal/features/pyramidize/ -run TestEvalLanguage -v -timeout 1800s
//
// EVAL_RUNS=n repeats each sample n times (default 3): 8 samples × 3 = 24
// calls. EVAL_PROVIDER / EVAL_MODEL as for TestEvalPyramidize. The replies go
// to test-data/eval-runs/<timestamp>-language/.
//
// Why it exists: on 2026-10-14 English notes came back in German in 6 of 7
// calls (v2: 3 of 3), and the main suite could not see it — its samples are
// all German. English input for every form, German for two, and one German
// mail with an English greeting and sign-off, which stays German. (The check
// does not ask whether the greeting stays English: a greeting has no function
// words to count.)
//
// The samples are written for this test, not taken from the prompt's examples
// or from the website's demo notes, and nothing in the prompt is tuned to them.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
)

var languageSamples = []struct{ name, form, text string }{
	{"email-en", "email", "hi Sam, quick one about the office move. the movers confirmed saturday the 12th, but the network team can only come on monday, so there is no internet on monday morning. could you tell everyone to work from home until noon? the new keys are at reception from friday. thanks, Jo"},
	{"chat-en", "chat", "hey, the staging deploy failed again with the same disk full error as last week. I cleared the old logs and restarted it, it should be back in about 10 minutes. can you hold off on merging until then?"},
	{"memo-en", "memo", "We need to tell the support team that from November 1 phone support moves to 8 to 6 instead of 9 to 5, because most calls come in before 9. Maria will update the hours on the website. Everyone has to check their shifts in the new rota by Friday and tell their lead if something does not work for them."},
	{"wiki-en", "wiki", "how to rotate the API signing key: generate a new key with `keytool rotate --env prod`, add it to the vault under signing/next, then wait an hour so that every instance has picked it up. after that run `keytool promote`. the old key stays valid for 24 hours. if anything breaks in that window, `keytool rollback` brings the old key back."},
	{"presentation-en", "powerpoint", "For the board: we cut customer churn from 8% to 5% this year by adding onboarding calls for every new account. That is about 300 customers we kept, worth roughly 450k in annual revenue. Next year we want to add in-app tutorials and bring churn down to 4%."},
	{"email-de", "email", "Hallo Kim, kurze Info zum Umzug: Die Spedition kommt am Samstag, dem 12., das Netzwerkteam aber erst am Montag. Montagvormittag haben wir also kein Internet. Kannst du allen sagen, dass sie bis Mittag von zu Hause arbeiten sollen? Die neuen Schlüssel liegen ab Freitag am Empfang. Danke, Jo"},
	{"chat-de", "chat", "Das Staging-Deployment ist wieder fehlgeschlagen, derselbe Fehler mit der vollen Platte wie letzte Woche. Ich habe die alten Logs gelöscht und neu gestartet, in etwa 10 Minuten sollte es wieder laufen. Kannst du bis dahin mit dem Mergen warten?"},
	{"email-mixed", "email", "Hi team, kurzes Update: der Kunde hat den Go-live auf den 5. Mai verschoben, weil ihr Legal Review noch läuft. Wir nutzen die Zeit für die Lasttests, die bisher zu kurz gekommen sind. Bitte plant eure Urlaube entsprechend und tragt sie bis Ende der Woche ins Team-Board ein. Cheers, Lea"},
}

func TestEvalLanguage(t *testing.T) {
	loadEvalEnv(t)

	// Isolated as TestEvalPyramidize is: an explicit configuration and
	// environment-only keys, so the GUI's settings cannot move the result.
	provider, model := evalTarget(os.Getenv)
	evalSettings := settings.Default()
	evalSettings.ActiveProvider = provider
	settingsSvc := settings.NewServiceFrom(evalSettings, settings.EnvOnlyKeys)
	if llm.UsesAPIKey(provider) && settingsSvc.GetKey(provider) == "" {
		t.Fatalf("no API key for %q in the environment or .env — the eval reads no keyring", provider)
	}
	model = llm.ResolveModel(context.Background(), provider, model,
		llm.Config{APIKey: settingsSvc.GetKey(provider), Feature: "eval"})

	runs := 3
	if v := os.Getenv("EVAL_RUNS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			t.Fatalf("EVAL_RUNS=%q: want a positive number", v)
		}
		runs = n
	}

	runDir := filepath.Join("..", "..", "..", "test-data", "eval-runs",
		time.Now().Format("2006-01-02T15-04-05")+"-language")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Logf("provider %s, model %s, %d run(s) per sample, replies in %s", provider, model, runs, runDir)

	svc := NewService(settingsSvc, nil)
	failed, total := 0, 0
	for _, s := range languageSamples {
		t.Run(s.name, func(t *testing.T) {
			if majorityLanguage(s.text) == "" {
				t.Fatal("the sample's own language cannot be told; it measures nothing")
			}
			for run := 1; run <= runs; run++ {
				result, err := svc.Pyramidize(PyramidizeRequest{
					Text:         s.text,
					DocumentType: s.form,
					Provider:     provider,
					Model:        model,
				})
				total++
				if err != nil {
					failed++
					t.Errorf("run %d: %v", run, err)
					continue
				}
				out := filepath.Join(runDir, fmt.Sprintf("%s-%d.md", s.name, run))
				if err := os.WriteFile(out, []byte(result.FullDocument), 0644); err != nil {
					t.Fatal(err)
				}
				if c := checkLanguage(s.text, result.FullDocument); !c.Pass {
					failed++
					t.Errorf("run %d: %s — %s", run, c.Detail, out)
				} else {
					t.Logf("run %d: %s", run, c.Detail)
				}
			}
		})
	}
	t.Logf("%d of %d replies in the author's language", total-failed, total)
}
