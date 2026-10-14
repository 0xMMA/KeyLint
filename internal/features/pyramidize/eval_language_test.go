package pyramidize

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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
