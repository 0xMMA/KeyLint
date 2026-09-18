package enhance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These run in the normal suite: the checks decide what the Fix eval reports,
// and an eval whose scoring is only exercised by a paid run is not maintainable.

// TestReturningTheInputScoresBadly is the property the whole suite rests on.
// Echoing the input is the cheapest wrong answer a model can give, and a
// scorecard that rewards it measures nothing.
func TestReturningTheInputScoresBadly(t *testing.T) {
	input := "the report was wrote by sarah and it dont include march"
	notes := SampleNotes{
		LanguageAnchors: []string{"report"},
		RequiredFixes: []fixPair{
			{From: "was wrote", To: "was written"},
			{From: "dont", To: "doesn't"},
		},
		MinLengthRatio: 0.9, MaxLengthRatio: 1.3,
	}

	card := RunDeterministicChecks(input, input, notes)

	if card.AllPassed {
		t.Error("echoing the input passed every check")
	}
	for _, c := range card.Checks {
		if c.Name == "required_fixes" && c.Score != 0 {
			t.Errorf("required_fixes = %.2f, want 0 — nothing was corrected", c.Score)
		}
	}
}

// TestTheAlreadyCorrectSampleWantsTheInputBack: the one place where changing
// nothing is the right answer, and where editing is the failure.
func TestTheAlreadyCorrectSampleWantsTheInputBack(t *testing.T) {
	input := "The quarterly figures are final."
	notes := SampleNotes{Unchanged: true, MinLengthRatio: 0.98, MaxLengthRatio: 1.02}

	if card := RunDeterministicChecks(input, input, notes); !card.AllPassed {
		t.Errorf("returning correct text unchanged failed a check: %+v", card.Checks)
	}

	edited := "The quarterly figures have been finalised."
	card := RunDeterministicChecks(input, edited, notes)
	if card.AllPassed {
		t.Error("rewriting already-correct text passed")
	}
}

func TestCommentaryIsCaught(t *testing.T) {
	for _, out := range []string{
		"Here's the corrected text:\nThe report is done.",
		"Corrected version:\nThe report is done.",
		"Sure! The report is done.",
		"I have corrected the errors.\nThe report is done.",
		"Note: two changes.\nThe report is done.",
	} {
		if c := checkNoCommentary(out); c.Pass {
			t.Errorf("commentary slipped through: %q", strings.SplitN(out, "\n", 2)[0])
		}
	}

	// "Note" mid-sentence is ordinary English and must not trip it.
	if c := checkNoCommentary("Please note the deadline moved to Friday."); !c.Pass {
		t.Errorf("a normal sentence was read as commentary: %s", c.Detail)
	}
}

// TestTrailingCommentaryIsCaught covers the shape the shipped prompt actually
// produces, observed in all three runs of the first baseline: the text comes
// back correct with a bracketed remark appended. The prefix pattern missed every
// one of them, which is how the gap was found.
func TestTrailingCommentaryIsCaught(t *testing.T) {
	for _, out := range []string{
		"The figures are final.\n\n(No corrections needed\u2014this text is grammatically correct and clear.)",
		"The figures are final. (No changes required.)",
		"Die Zahlen stehen. [keine Korrekturen nötig]",
	} {
		if c := checkNoCommentary(out); c.Pass {
			t.Errorf("trailing commentary slipped through: %q", out)
		}
	}

	// Parentheses that are part of the text must survive.
	for _, out := range []string{
		"The launch (originally Friday) moved to Monday.",
		"Wir treffen uns um 9 Uhr (wie besprochen).",
		"Send it to the team (Sarah, Tom) before noon.",
	} {
		if c := checkNoCommentary(out); !c.Pass {
			t.Errorf("real content was read as commentary: %q — %s", out, c.Detail)
		}
	}
}

func TestTranslationIsCaught(t *testing.T) {
	notes := SampleNotes{
		LanguageAnchors: []string{"Termin", "Protokoll"},
		Forbidden:       []string{"appointment", "minutes"},
	}
	if c := checkLanguagePreserved("Der Termin steht, das Protokoll folgt.", notes); !c.Pass {
		t.Errorf("a correct German output was flagged: %s", c.Detail)
	}
	if c := checkLanguagePreserved("The appointment is set, the minutes follow.", notes); c.Pass {
		t.Error("a full translation passed the language check")
	}
}

func TestMarkdownFlatteningIsCaught(t *testing.T) {
	input := "## Notes\n\n- one\n- two\n\n**important**: `run.sh`"
	if c := checkMarkdownPreserved(input, input); !c.Pass {
		t.Errorf("unchanged markdown failed: %s", c.Detail)
	}
	flat := "Notes. One, two. Important: run.sh"
	if c := checkMarkdownPreserved(input, flat); c.Pass {
		t.Error("flattened markdown passed")
	}
}

// TestEverySampleIsUsable parses all fifteen and refuses the two shapes that
// would make a sample worthless: nothing to enforce, and a reference that does
// not itself satisfy the checks.
func TestEverySampleIsUsable(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "test-data", "fix-samples")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", dir, err)
	}

	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		count++
		t.Run(e.Name(), func(t *testing.T) {
			base := filepath.Join(dir, e.Name())
			notes, err := parseNotes(filepath.Join(base, "notes.md"))
			if err != nil {
				t.Fatalf("notes: %v", err)
			}
			input := readTrimmed(t, filepath.Join(base, "input.md"))
			reference := readTrimmed(t, filepath.Join(base, "reference.md"))

			// The human reference has to pass the checks it is the answer to.
			// If it does not, the checks are wrong, not the model.
			card := RunDeterministicChecks(input, reference, notes)
			if !card.AllPassed {
				for _, c := range card.Checks {
					if !c.Pass {
						t.Errorf("the reference fails %s: %s", c.Name, c.Detail)
					}
				}
			}

			// And the input must NOT already pass, or the sample asks nothing.
			if !notes.Unchanged {
				if RunDeterministicChecks(input, input, notes).AllPassed {
					t.Error("the unedited input passes every check, so this sample tests nothing")
				}
			}
		})
	}
	if count != 15 {
		t.Errorf("found %d samples, expected 15", count)
	}
}

func readTrimmed(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return strings.TrimSpace(string(data))
}

// TestFencedJudgeRepliesParse: the judge is told to answer with JSON only and
// the request asks for JSON mode, and it still wraps the object in a markdown
// fence. The first baseline run lost every judge score to this, so the shapes
// below are the ones actually observed plus the plain case.
func TestFencedJudgeRepliesParse(t *testing.T) {
	const obj = `{"correctness":0.9,"meaningPreserved":1,"tonePreserved":1,"noOverEditing":0.8,"overall":0.9,"rationale":"ok"}`

	for name, raw := range map[string]string{
		"plain":            obj,
		"json fence":       "```json\n" + obj + "\n```",
		"bare fence":       "```\n" + obj + "\n```",
		"fence with space": "  ```json\n" + obj + "\n```  ",
	} {
		if got := stripCodeFence(raw); got != obj {
			t.Errorf("%s: got %q", name, got)
		}
	}
}

// TestACorrectionCountsHoweverItIsCapitalised: `ausserdem->außerdem` used to
// fail against an output that split the sentence and wrote "Außerdem" at the
// start of the new one — the model was more right than the note, and a literal
// substring test called it wrong.
func TestACorrectionCountsHoweverItIsCapitalised(t *testing.T) {
	notes := SampleNotes{RequiredFixes: []fixPair{{From: "ausserdem", To: "außerdem"}}}

	for _, out := range []string{
		"Die Größe fehlt, außerdem wäre es gut.",
		"Die Größe fehlt. Außerdem wäre es gut.",
	} {
		if c := checkRequiredFixes(out, notes); !c.Pass {
			t.Errorf("%q was scored as unfixed: %s", out, c.Detail)
		}
	}

	if c := checkRequiredFixes("Die Größe fehlt, ausserdem waere es gut.", notes); c.Pass {
		t.Error("the uncorrected spelling passed")
	}
}

// TestACasingFixStillNeedsTheCasing: the exception. When a correction is only
// about capitalisation, matching case-insensitively would accept the error.
func TestACasingFixStillNeedsTheCasing(t *testing.T) {
	notes := SampleNotes{RequiredFixes: []fixPair{{From: "termin", To: "Termin"}}}

	if c := checkRequiredFixes("Der Termin steht.", notes); !c.Pass {
		t.Errorf("a correct capitalisation was rejected: %s", c.Detail)
	}
	if c := checkRequiredFixes("Der termin steht.", notes); c.Pass {
		t.Error("the lowercase noun passed a capitalisation fix")
	}
}

// TestAFixWhoseWrongFormIsInsideTheRightOne: `ich hab->ich habe` cannot be
// checked by demanding the wrong form is gone, because it never is.
func TestAFixWhoseWrongFormIsInsideTheRightOne(t *testing.T) {
	notes := SampleNotes{RequiredFixes: []fixPair{{From: "ich hab", To: "ich habe"}}}

	if c := checkRequiredFixes("Ich habe den Branch gemerged.", notes); !c.Pass {
		t.Errorf("the corrected form was rejected: %s", c.Detail)
	}
	if c := checkRequiredFixes("Ich hab den Branch gemerged.", notes); c.Pass {
		t.Error("the uncorrected form passed")
	}
}
