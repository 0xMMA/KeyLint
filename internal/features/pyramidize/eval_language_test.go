package pyramidize

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readLanguageFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "language", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestCheckLanguageCatchesTranslatedReplies: the fixtures are real replies of
// 2026-10-14 to English notes (Sonnet through the Claude Code CLI, before the
// fix): translated whole, translated with English words left in, and English
// bullets under German subject and headers. All three fail; an English reply
// to the same notes and a German reply to German input pass.
func TestCheckLanguageCatchesTranslatedReplies(t *testing.T) {
	notes := readLanguageFixture(t, "en-notes.in.txt")
	for _, tc := range []struct {
		out  string
		pass bool
	}{
		{"en-notes.out-translated.md", false},
		{"en-notes.out-mixed.md", false},
		{"en-notes.out-headers.md", false},
		{"en-notes.out-ok.md", true},
	} {
		got := checkLanguage(notes, readLanguageFixture(t, tc.out))
		if got.Pass != tc.pass {
			t.Errorf("%s: pass = %v, want %v (%s)", tc.out, got.Pass, tc.pass, got.Detail)
		}
	}
	got := checkLanguage(readLanguageFixture(t, "de-kpi.in.txt"), readLanguageFixture(t, "de-kpi.out-ok.md"))
	if !got.Pass {
		t.Errorf("German reply to German input fails: %s", got.Detail)
	}
}

// TestCheckLanguagePassesTheAcceptedOutputs: every output the user accepted
// for a Pyramidize eval sample is in the sample's language — German, with the
// English terms its author used. A check that failed one of them would report
// the author's own mix as a translation.
func TestCheckLanguagePassesTheAcceptedOutputs(t *testing.T) {
	samples, _, err := loadEvalSamples(filepath.Join("..", "..", "..", "test-data", "pyramidal-emails"))
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) == 0 {
		t.Fatal("no eval samples found")
	}
	for _, s := range samples {
		if s.Baseline == "" {
			t.Errorf("%s has no accepted output, so its language goes unchecked", s.Name)
			continue
		}
		if got := checkLanguage(s.AuthorText, s.Baseline); !got.Pass {
			t.Errorf("%s: %s", s.Name, got.Detail)
		}
	}
}

var exampleInput = regexp.MustCompile(`(?m)^input: "(.*)"$`)

// TestCheckLanguagePassesThePromptExamples: each v3 example reply is in the
// language of its input, the code-switching ones included ("Q1 Review Monday
// 2pm statt Friday" was v2's; "Hi Tom, kurzes Update" is chat's).
func TestCheckLanguagePassesThePromptExamples(t *testing.T) {
	for _, form := range forms {
		section := promptV3Forms[form]
		inputs := exampleInput.FindAllStringSubmatch(section, -1)
		outputs := exampleOutput.FindAllStringSubmatch(section, -1)
		if len(inputs) == 0 || len(inputs) != len(outputs) {
			t.Fatalf("%s: %d example inputs, %d outputs", form, len(inputs), len(outputs))
		}
		for i := range inputs {
			var reply struct {
				FullDocument string `json:"fullDocument"`
			}
			if err := json.Unmarshal([]byte(outputs[i][1]), &reply); err != nil {
				t.Fatal(err)
			}
			if got := checkLanguage(inputs[i][1], reply.FullDocument); !got.Pass {
				t.Errorf("%s example %d: %s", form, i+1, got.Detail)
			}
		}
	}
}

// languageSamples are TestEvalLanguage's inputs (eval tag). They live here so
// that a unit test can check each one is what it claims, without a call.
// Written for that test, not taken from the prompt's examples or the
// website's demo notes; nothing in the prompt is tuned to them.
var languageSamples = []struct {
	name, form, text string
	lang             string // the language the text is written in
	instructions     string // the user's instructions, if any
	want             string // the language the reply must be in
}{
	{"email-en", "email", "hi Sam, quick one about the office move. the movers confirmed saturday the 12th, but the network team can only come on monday, so there is no internet on monday morning. could you tell everyone to work from home until noon? the new keys are at reception from friday. thanks, Jo", "en", "", "en"},
	{"chat-en", "chat", "hey, the staging deploy failed again with the same disk full error as last week. I cleared the old logs and restarted it, it should be back in about 10 minutes. can you hold off on merging until then?", "en", "", "en"},
	{"memo-en", "memo", "We need to tell the support team that from November 1 phone support moves to 8 to 6 instead of 9 to 5, because most calls come in before 9. Maria will update the hours on the website. Everyone has to check their shifts in the new rota by Friday and tell their lead if something does not work for them.", "en", "", "en"},
	{"wiki-en", "wiki", "how to rotate the API signing key: generate a new key with `keytool rotate --env prod`, add it to the vault under signing/next, then wait an hour so that every instance has picked it up. after that run `keytool promote`. the old key stays valid for 24 hours. if anything breaks in that window, `keytool rollback` brings the old key back.", "en", "", "en"},
	{"presentation-en", "powerpoint", "For the board: we cut customer churn from 8% to 5% this year by adding onboarding calls for every new account. That is about 300 customers we kept, worth roughly 450k in annual revenue. Next year we want to add in-app tutorials and bring churn down to 4%.", "en", "", "en"},
	{"email-de", "email", "Hallo Kim, kurze Info zum Umzug: Die Spedition kommt am Samstag, dem 12., das Netzwerkteam aber erst am Montag. Montagvormittag haben wir also kein Internet. Kannst du allen sagen, dass sie bis Mittag von zu Hause arbeiten sollen? Die neuen Schlüssel liegen ab Freitag am Empfang. Danke, Jo", "de", "", "de"},
	{"chat-de", "chat", "Das Staging-Deployment ist wieder fehlgeschlagen, derselbe Fehler mit der vollen Platte wie letzte Woche. Ich habe die alten Logs gelöscht und neu gestartet, in etwa 10 Minuten sollte es wieder laufen. Kannst du bis dahin mit dem Mergen warten?", "de", "", "de"},
	{"email-mixed", "email", "Hi team, kurzes Update: der Kunde hat den Go-live auf den 5. Mai verschoben, weil ihr Legal Review noch läuft. Wir nutzen die Zeit für die Lasttests, die bisher zu kurz gekommen sind. Bitte plant eure Urlaube entsprechend und tragt sie bis Ende der Woche ins Team-Board ein. Cheers, Lea", "de", "", "de"},
	// The user's instructions may ask for another language: the UI's own
	// placeholder suggests "in English". The reply names the language it is
	// written in, so language-first must not pin it to the author's.
	{"email-de-in-english", "email", "Hallo Ben, die Lizenzen für das neue Design-Tool laufen am 30. Juni aus. Die Verlängerung kostet 2.400 Euro für das Jahr, das liegt noch im Budget. Ich würde gern bis Ende Mai verlängern, damit niemand ohne Zugang dasteht. Gibst du mir bis Freitag dein Okay? Danke, Mara", "de", "in English", "en"},
}

// TestLanguageSamplesAreWhatTheyClaim: a sample whose own language the check
// cannot tell, or tells wrongly, measures nothing.
func TestLanguageSamplesAreWhatTheyClaim(t *testing.T) {
	forms := map[string]bool{"email": true, "chat": true, "memo": true, "wiki": true, "powerpoint": true}
	for _, s := range languageSamples {
		if got := majorityLanguage(s.text); got != s.lang {
			t.Errorf("%s: written in %q, the check reads %q", s.name, s.lang, got)
		}
		if s.instructions == "" && s.want != s.lang {
			t.Errorf("%s: no instructions, yet the reply should be %q, not %q", s.name, s.want, s.lang)
		}
		if !forms[s.form] {
			t.Errorf("%s: unknown form %q", s.name, s.form)
		}
	}
}

// Review of #132: short forms and abbreviations are not running text. "Do"
// is German for Thursday, "i.A." and "To-do" split into "i", "a", "to"; a
// casual English "im" is "I'm". None of them may turn a right reply wrong.
func TestCheckLanguageIgnoresShortFormsAndAbbreviations(t *testing.T) {
	de := "Hallo Kim, die Spedition kommt am Samstag, das Netzwerkteam aber erst am Montag. Kannst du allen sagen, dass sie bis Mittag von zu Hause arbeiten? Die Schlüssel liegen ab Freitag am Empfang."
	deReply := "Hi Kim,\nDie Spedition kommt am Samstag, das Netzwerk erst am Montag.\nBitte sag allen, dass sie bis Mittag von zu Hause arbeiten.\nDie Schlüssel liegen ab Freitag am Empfang.\nTo-do: Merge am Do\ni.A. Jo"
	if got := checkLanguage(de, deReply); !got.Pass {
		t.Errorf("German reply with \"To-do: Merge am Do\" and \"i.A.\" fails: %s", got.Detail)
	}
	en := "hi Sam, the movers come on saturday but the network team only on monday. could you tell everyone to work from home until noon? im out tomorrow."
	enReply := "Movers on Saturday, network on Monday | @all work from home until noon\n\nhi Sam,\n\n**There is no internet on Monday morning**\n- The movers come on Saturday, the network team only on Monday\n- Please work from home until noon\n- im out tomorrow"
	if got := checkLanguage(en, enReply); !got.Pass {
		t.Errorf("English reply with \"im out tomorrow\" fails: %s", got.Detail)
	}
}

// A translated subject is one line among many, under the share limit, and
// still the first thing every reader sees.
func TestCheckLanguageHoldsTheFirstLine(t *testing.T) {
	en := "hi Sam, the movers come on saturday but the network team only on monday. could you tell everyone to work from home until noon? I am out tomorrow, so ask Jo if anything comes up."
	reply := "Umzug: das Netzwerk kommt erst am Montag, bis Mittag von zu Hause arbeiten\n\nhi Sam,\n\n" +
		"**There is no internet on Monday morning**\n" +
		"- The movers come on Saturday, but the network team is only there on Monday\n" +
		"- Could you tell everyone to work from home until noon?\n" +
		"- I am out tomorrow, so please ask Jo if anything comes up\n" +
		"- The keys are at the reception from Friday\n" +
		"- If the network is up early, I will let you know\n" +
		"- Thanks for passing this on to the team"
	got := checkLanguage(en, reply)
	if got.Pass {
		t.Errorf("an English reply under a German subject passes: %s", got.Detail)
	}
	if !strings.Contains(got.Detail, "first line de") {
		t.Errorf("detail does not name the first line: %s", got.Detail)
	}
}

// checkLanguageAs is for instructions that ask for another language: an
// English reply to German input passes it and fails checkLanguage.
func TestCheckLanguageAsFollowsTheInstructions(t *testing.T) {
	de := readLanguageFixture(t, "de-kpi.in.txt")
	en := readLanguageFixture(t, "en-notes.out-ok.md")
	if got := checkLanguageAs("en", en); !got.Pass {
		t.Errorf("English reply, English asked for: %s", got.Detail)
	}
	if got := checkLanguage(de, en); got.Pass {
		t.Errorf("English reply to German input, nothing asked for, passes: %s", got.Detail)
	}
	if got := checkLanguageAs("en", readLanguageFixture(t, "de-kpi.out-ok.md")); got.Pass {
		t.Errorf("German reply, English asked for, passes: %s", got.Detail)
	}
}

func TestCheckLanguageLeavesShortInputAlone(t *testing.T) {
	if got := checkLanguage("ok, Freitag", "Ja, Freitag passt."); !got.Pass {
		t.Errorf("a three-word input cannot be told: %s", got.Detail)
	}
}

// IT, US and EUR are acronyms, not the pronoun "it" or "us": an English
// reply about the IT pool must not look German, nor a German one English.
func TestLanguageCountsSkipAcronyms(t *testing.T) {
	if de, en := languageCounts("IT US EUR"); de != 0 || en != 0 {
		t.Errorf("acronyms counted: de=%d en=%d", de, en)
	}
	if de, en := languageCounts("I need it"); en != 2 || de != 0 {
		t.Errorf(`"I need it": de=%d en=%d, want en=2 ("I" and "it")`, de, en)
	}
}
