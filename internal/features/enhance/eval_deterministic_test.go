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

	card := RunDeterministicChecks(input, input, input, notes)

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

	if card := RunDeterministicChecks(input, input, input, notes); !card.AllPassed {
		t.Errorf("returning correct text unchanged failed a check: %+v", card.Checks)
	}

	edited := "The quarterly figures have been finalised."
	card := RunDeterministicChecks(input, edited, edited, notes)
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
			card := RunDeterministicChecks(input, reference, reference, notes)
			if !card.AllPassed {
				for _, c := range card.Checks {
					if !c.Pass {
						t.Errorf("the reference fails %s: %s", c.Name, c.Detail)
					}
				}
			}

			// And the input must NOT already pass, or the sample asks nothing.
			// Scored against its real reference: passing the input as its own
			// reference made resembles_reference compare a text with itself,
			// which is not the question the suite asks of a model.
			if !notes.Unchanged {
				if RunDeterministicChecks(input, reference, input, notes).AllPassed {
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

// buildSalad is the cheapest wrong answer that is not an echo: every string the
// notes expect, joined by noise and padded into the length band.
func buildSalad(input string, n SampleNotes) string {
	var parts []string
	for _, f := range n.RequiredFixes {
		parts = append(parts, f.To)
	}
	parts = append(parts, n.LanguageAnchors...)
	parts = append(parts, n.ToneAnchors...)
	salad := strings.Join(parts, " xx ")
	for len([]rune(salad)) < int(float64(len([]rune(input)))*0.95) {
		salad += " zz"
	}
	return salad
}

// TestWordSaladFailsEverySample is the property the first version of this file
// claimed and did not have. A review built exactly this output and found it
// scored 1.000 with every check passing on 13 of the 15 samples: presence tests
// and a rune count cannot tell prose from a bag of the expected words.
func TestWordSaladFailsEverySample(t *testing.T) {
	forEachSample(t, func(t *testing.T, name, input, reference string, n SampleNotes) {
		card := RunDeterministicChecks(input, reference, buildSalad(input, n), n)
		if card.AllPassed {
			t.Errorf("a bag of the expected strings passed every check")
		}
		if card.OverallScore > 0.9 {
			t.Errorf("word salad scored %.3f — too close to a real correction", card.OverallScore)
		}
	})
}

// TestDeletingContentFails: padding a truncation back to the right length used
// to restore a perfect score, because nothing looked at what had been removed.
func TestDeletingContentFails(t *testing.T) {
	input := "wir haben die lizenzkosten geprüft, der rollout verzögert sich um sechs wochen, " +
		"die einsparungen greifen erst im nächsten quartal."
	reference := "Wir haben die Lizenzkosten geprüft. Der Rollout verzögert sich um sechs Wochen, " +
		"die Einsparungen greifen erst im nächsten Quartal."
	notes := SampleNotes{
		RequiredFixes:   []fixPair{{From: "lizenzkosten", To: "Lizenzkosten"}},
		LanguageAnchors: []string{"Rollout"},
		MinLengthRatio:  0.9, MaxLengthRatio: 1.3,
	}

	// Two of the three facts dropped, padded back into the band.
	truncated := "Wir haben die Lizenzkosten geprüft. Der Rollout kommt. Das ist so. Das ist so. Das ist so."
	card := RunDeterministicChecks(input, reference, truncated, notes)
	if card.AllPassed {
		t.Error("an output that dropped most of the content passed every check")
	}
}

// forEachSample runs f over the committed samples.
func forEachSample(t *testing.T, f func(t *testing.T, name, input, reference string, n SampleNotes)) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "test-data", "fix-samples")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		base := filepath.Join(dir, e.Name())
		notes, err := parseNotes(filepath.Join(base, "notes.md"))
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		t.Run(e.Name(), func(t *testing.T) {
			f(t, e.Name(), readTrimmed(t, filepath.Join(base, "input.md")),
				readTrimmed(t, filepath.Join(base, "reference.md")), notes)
		})
	}
}

// TestTrailingCommentaryEvasions: the shapes a review found slipping past the
// first pattern, including the one that actually occurred inside the baseline
// this suite recorded — the same sentence wrapped in markdown emphasis.
func TestTrailingCommentaryEvasions(t *testing.T) {
	for _, out := range []string{
		"The figures are final.\n\n*(No corrections needed—this text is grammatically correct and clear.)*",
		"The figures are final. (No changes required.)",
		"The figures are final.\n\n— No corrections needed, this text is already correct.",
		"The figures are final.\n\n**No corrections needed.**",
		"The figures are final.\n\nThe text was already correct, so nothing was changed.",
		"Die Zahlen stehen.\n\nEs waren keine Korrekturen nötig.",
		"Die Zahlen stehen. [keine Korrekturen nötig]",
	} {
		if c := checkNoCommentary(out); c.Pass {
			t.Errorf("evasion slipped through: %q", out)
		}
	}

	// Real text that merely mentions changes must survive.
	for _, out := range []string{
		"We discussed the changes needed for the launch.",
		"The launch (originally Friday) moved to Monday.",
		"Die Änderungen am Vertrag sind durch.",
		"Send it to the team (Sarah, Tom) before noon.",
	} {
		if c := checkNoCommentary(out); !c.Pass {
			t.Errorf("real content was read as commentary: %q — %s", out, c.Detail)
		}
	}
}

// TestForbiddenWordsMatchOnBoundaries: a substring test reported a correct
// German output as a translation because "Dienst" is inside "Dienstag".
func TestForbiddenWordsMatchOnBoundaries(t *testing.T) {
	notes := SampleNotes{LanguageAnchors: []string{"Service"}, Forbidden: []string{"Dienst"}}

	if c := checkLanguagePreserved("Der Service läuft seit Dienstag wieder.", notes); !c.Pass {
		t.Errorf("a weekday was read as a translation: %s", c.Detail)
	}
	if c := checkLanguagePreserved("Der Dienst läuft wieder.", notes); c.Pass {
		t.Error("the actual translation passed")
	}
}

// TestTheUnfixedInputFailsEverySample is the third entry in the evasion table,
// next to word salad and a padded truncation — and the one that was missing.
//
// It is the cheapest wrong answer of all, and `resembles_reference` was blind to
// it: token F1 lowercases and strips punctuation, so on a sample whose
// corrections are capitals or commas the input and the reference had identical
// token bags. Measured before the fix, the raw input cleared the 0.70 floor on
// 14 of the 15 samples and scored a flat 1.000 on five of them.
func TestTheUnfixedInputFailsEverySample(t *testing.T) {
	forEachSample(t, func(t *testing.T, name, input, reference string, n SampleNotes) {
		card := RunDeterministicChecks(input, reference, input, n)
		if len(n.RequiredFixes) == 0 {
			// schon-korrekt: returning the input IS the answer here.
			if !card.AllPassed {
				t.Errorf("the already-correct sample rejected its own input: %+v", card.Checks)
			}
			return
		}
		if card.AllPassed {
			t.Error("the unfixed input passed every check")
		}
		for _, c := range card.Checks {
			if c.Name == "resembles_reference" && c.Pass {
				t.Errorf("resembles_reference passed the unfixed input: %s", c.Detail)
			}
		}
	})
}

// TestResemblanceIsRelativeToTheInput states the property directly: a
// correction is closer to the corrected text than to the text it corrects.
// An absolute floor cannot express that — on a caps-only fix the unfixed input
// scores 0.97 against the reference, which is a fine number and the wrong
// answer.
func TestResemblanceIsRelativeToTheInput(t *testing.T) {
	const (
		input     = "der termin steht, das protokoll folgt."
		reference = "Der Termin steht, das Protokoll folgt."
	)
	notes := SampleNotes{RequiredFixes: []fixPair{{From: "termin", To: "Termin"}}}

	if c := checkResemblesReference(input, reference, reference, notes); !c.Pass {
		t.Errorf("the reference itself failed: %s", c.Detail)
	}
	if c := checkResemblesReference(input, reference, input, notes); c.Pass || c.Score != 0 {
		t.Errorf("the unfixed input scored %.3f (pass=%v): %s", c.Score, c.Pass, c.Detail)
	}
	// A correction that differs from the reference in wording is still a
	// correction, and must not be punished for it.
	other := "Der Termin steht; das Protokoll folgt."
	if c := checkResemblesReference(input, reference, other, notes); !c.Pass {
		t.Errorf("a differently-worded correction failed: %s", c.Detail)
	}
}

// TestLeadingCommentaryIsCaught: the trailing pattern anchored on `$` without
// (?m). In Go that means end of TEXT — and, unlike Perl, not before a final
// newline. So a remark with anything after it escaped, and so did the exact
// shape the baseline documents as happening in every run, as soon as the output
// ended with a newline.
func TestLeadingCommentaryIsCaught(t *testing.T) {
	for _, out := range []string{
		"(No corrections needed.)\nThe figures are final.",
		"No corrections needed — the text is already correct.\n\nThe figures are final.",
		"Keine Korrekturen nötig.\n\nDie Zahlen stehen.",
		"The figures are final. (No corrections needed.)\n",
		"The figures are final. (No corrections needed.)\n\n",
		"Die Zahlen stehen. (Keine Änderungen notwendig.)\n",
		"The figures are final. (No changes necessary.)",
		"Der Satz ist schon korrekt.",
	} {
		if c := checkNoCommentary(out); c.Pass {
			t.Errorf("commentary slipped through: %q", out)
		}
	}

	// A trailing newline on clean text is not commentary.
	for _, out := range []string{
		"The figures are final.\n",
		"Die Änderungen am Vertrag sind durch.\n\n",
	} {
		if c := checkNoCommentary(out); !c.Pass {
			t.Errorf("clean text was read as commentary: %q — %s", out, c.Detail)
		}
	}
}

// TestACasingFixNeedsTheWrongCasingGone: presence alone accepted an output that
// capitalised the second occurrence and left the first as it was.
func TestACasingFixNeedsTheWrongCasingGone(t *testing.T) {
	notes := SampleNotes{RequiredFixes: []fixPair{{From: "standup", To: "Standup"}}}

	if c := checkRequiredFixes("Das Standup bleibt, wie es ist.", notes); !c.Pass {
		t.Errorf("a fully corrected output was rejected: %s", c.Detail)
	}
	if c := checkRequiredFixes("Das standup bleibt; das Standup ist kurz.", notes); c.Pass {
		t.Error("an output still carrying the lowercase form passed")
	}
}

// TestAnInflectedWordIsNotLostContent: a correction changes endings, and
// content_retained compared exact word forms. An output that correctly wrote
// "schicke" for the author's "schick" was scored as having dropped a word.
func TestAnInflectedWordIsNotLostContent(t *testing.T) {
	input := "ich schick nachher noch die agenda rum, das meeting bleibt"
	notes := SampleNotes{RequiredFixes: []fixPair{{From: "das meeting", To: "Das Meeting"}}}

	out := "Ich schicke nachher noch die Agenda rum, das Meeting bleibt"
	c := checkContentRetained(input, out, notes)
	if !c.Pass || c.Score != 1 {
		t.Errorf("an inflected correction was scored as content loss: %.3f — %s", c.Score, c.Detail)
	}

	// The tolerance is two runes of ending, not a free pass: a different word
	// is still a different word.
	if c := checkContentRetained(input, "Ich verschicke nachher nichts, das Meeting bleibt", notes); c.Pass {
		t.Errorf("an output that dropped content passed: %s", c.Detail)
	}
}

// TestForbiddenWordsMatchInflections: the anglicism samples forbid the German
// translation, and a model that wrote "Besprechungen" walked past the check
// because only the exact form was searched for. The endings are a closed list,
// not a prefix test — "Dienstag" must still not read as "Dienst".
func TestForbiddenWordsMatchInflections(t *testing.T) {
	notes := SampleNotes{
		LanguageAnchors: []string{"Meeting"},
		Forbidden:       []string{"Besprechung", "Dienst"},
	}

	for _, out := range []string{
		"Wir verschieben die Besprechung.",
		"Wir verschieben die Besprechungen.",
		"Die Dienste laufen wieder.",
	} {
		notes.LanguageAnchors = nil // the anchor is not the point here
		if c := checkLanguagePreserved(out, notes); c.Pass {
			t.Errorf("a translation passed: %q", out)
		}
	}

	notes.LanguageAnchors = []string{"Meeting"}
	for _, out := range []string{
		"Das Meeting ist am Dienstag.",
		"Das Meeting läuft seit Dienstagmorgen.",
	} {
		if c := checkLanguagePreserved(out, notes); !c.Pass {
			t.Errorf("a weekday was read as a translation: %q — %s", out, c.Detail)
		}
	}
}

// TestOrdinaryProseIsNotCommentary: `here( is)?` matched "Here is the report you
// asked for." — a sentence a user might type and want corrected, scored as if
// the model had narrated its own work. What makes the phrase commentary is what
// comes after it.
func TestOrdinaryProseIsNotCommentary(t *testing.T) {
	for _, out := range []string{
		"Here is the report you asked for.",
		"Here are the numbers for the third quarter.",
		"Here's the link to the dashboard, let me know if it loads.",
	} {
		if c := checkNoCommentary(out); !c.Pass {
			t.Errorf("ordinary prose was read as commentary: %q — %s", out, c.Detail)
		}
	}

	for _, out := range []string{
		"Here's the corrected text:\nThe report is done.",
		"Here is the corrected version:\nThe report is done.",
		"The figures are final.\n\n(Everything looks good.)",
		"Die Zahlen stehen. (Keine Änderungen vorgenommen.)",
	} {
		if c := checkNoCommentary(out); c.Pass {
			t.Errorf("commentary slipped through: %q", out)
		}
	}
}

// TestPunctuationIsCounted: the deterministic layer was blind to it. Applying
// only the must-change replacements to the raw input passed every check on 8 of
// the 15 samples, and the sample whose entire purpose is punctuation scored
// 1.000 with none in it — the word-level checks strip punctuation and 15
// characters barely move an edit distance over 120.
func TestPunctuationIsCounted(t *testing.T) {
	const (
		input     = "wenn der kunde nicht antwortet rufen wir an ansonsten warten wir"
		reference = "Wenn der Kunde nicht antwortet, rufen wir an. Ansonsten warten wir."
	)

	if c := checkPunctuationRestored(input, reference, reference); !c.Pass {
		t.Errorf("the reference failed its own check: %s", c.Detail)
	}
	if c := checkPunctuationRestored(input, reference, "Wenn der Kunde nicht antwortet rufen wir an ansonsten warten wir"); c.Pass {
		t.Error("an unpunctuated output passed")
	}
	// Where a boundary goes is a judgement call: one sentence where the
	// reference wrote two still restored the boundary that was missing.
	if c := checkPunctuationRestored(input, reference, "Wenn der Kunde nicht antwortet, rufen wir an, ansonsten warten wir."); !c.Pass {
		t.Errorf("a differently-segmented correction failed: %s", c.Detail)
	}
	// A full stop where the reference used a comma is the same boundary. The
	// first version of this check counted the two separately and called this
	// correct German a lost comma — measured, in all three baseline runs of
	// de-umlaute-ascii.
	if c := checkPunctuationRestored(
		"fuer die dateien, ausserdem waere es gut wenn wir die pruefung vorziehen koennten.",
		"Für die Dateien, außerdem wäre es gut, wenn wir die Prüfung vorziehen könnten.",
		"Für die Dateien. Außerdem wäre es gut, wenn wir die Prüfung vorziehen könnten.",
	); !c.Pass {
		t.Errorf("a full stop in place of the reference's comma was scored as missing punctuation: %s", c.Detail)
	}
	// A sample whose input is already punctuated asks nothing here.
	if c := checkPunctuationRestored(reference, reference, reference); !c.Pass || c.Detail != "nothing to restore" {
		t.Errorf("an already-punctuated sample was scored: %s", c.Detail)
	}
}

// TestSentenceCapitalsAreCounted covers the correction that `must-change` can no
// longer name: a pair like `das meeting->Das Meeting` bundled the noun's capital
// with the article's, and failed an output that wrote "Wir verschieben das
// Meeting" — correct German, rejected for not repeating the reference's sentence
// structure.
func TestSentenceCapitalsAreCounted(t *testing.T) {
	const reference = "Der Termin steht. Das Protokoll folgt."

	if c := checkSentenceCapitals(reference, reference); !c.Pass {
		t.Errorf("the reference failed: %s", c.Detail)
	}
	if c := checkSentenceCapitals(reference, "Der Termin steht. das Protokoll folgt."); c.Pass {
		t.Error("a lower-case sentence start passed")
	}
	// Counted against the reference, not against zero: an abbreviation or a
	// deliberately lower-case opening costs nothing when both sides have it.
	if c := checkSentenceCapitals("z.B. dies hier.", "z.B. dies hier."); !c.Pass {
		t.Errorf("an abbreviation was read as a missing capital: %s", c.Detail)
	}
	// A filename is not a sentence boundary.
	if c := checkSentenceCapitals("See `docs/upgrade.md` for details.", "See `docs/upgrade.md` for details."); !c.Pass {
		t.Errorf("a filename was read as a sentence boundary: %s", c.Detail)
	}
}

// TestARewriteIsNotACorrection: over-editing was invisible outside the
// already-correct sample. A chat message rewritten into different words keeps
// the facts and can stay near the reference; what gives it away is the distance
// from what the author actually wrote.
func TestARewriteIsNotACorrection(t *testing.T) {
	const (
		input     = "hey, kannst du kurz draufschauen? hab da was gebastelt aber bin mir nicht sicher ob das so passt."
		reference = "Hey, kannst du kurz draufschauen? Hab da was gebastelt, aber bin mir nicht sicher, ob das so passt."
	)
	notes := SampleNotes{RequiredFixes: []fixPair{{From: "hey,", To: "Hey,"}}}

	if c := checkResemblesReference(input, reference, reference, notes); !c.Pass {
		t.Errorf("the reference failed: %s", c.Detail)
	}
	// Both of these are far enough from the reference to fail the floor as
	// well. The drift cap is what names them: "rewritten rather than
	// corrected" is a different diagnosis from "does not look like the
	// reference", and it is the one that is true.
	for _, rewrite := range []string{
		"Hallo, könntest du bitte kurz einen Blick darauf werfen? Ich habe etwas vorbereitet, bin mir aber nicht sicher, ob es passt.",
		"Hallo, könnten Sie bitte kurz darauf schauen? Ich habe etwas erstellt, bin mir aber nicht sicher, ob es so passt.",
	} {
		c := checkResemblesReference(input, reference, rewrite, notes)
		if c.Pass {
			t.Errorf("a rewrite passed as a correction: %s", c.Detail)
		}
		if !strings.Contains(c.Detail, "rewritten rather than corrected") {
			t.Errorf("a rewrite was diagnosed as %q", c.Detail)
		}
	}
}

// TestNotesRejectWhatCannotBeChecked: a typo in a key used to disable the check
// it configures in silence, and a casing fix spanning several words bakes the
// reference's sentence structure into a pair about one word's capital.
func TestNotesRejectWhatCannotBeChecked(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) string {
		path := filepath.Join(dir, strings.ReplaceAll(body[:12], " ", "_")+".md")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	if _, err := parseNotes(write("forbiden: Besprechung\nmust-change: a->b\n")); err == nil {
		t.Error("a misspelled key was accepted")
	}
	if _, err := parseNotes(write("must-change: das meeting->Das Meeting\n")); err == nil {
		t.Error("a multi-word casing fix was accepted")
	}
	if _, err := parseNotes(write("language: de\nmust-change: meeting->Meeting\nwhy: prose\n")); err != nil {
		t.Errorf("a valid notes file was rejected: %v", err)
	}
}

// TestTheMinimalReplacementFailsEverySample is the fourth entry in the evasion
// table, and the one that found the most: apply ONLY the `must-change`
// replacements to the raw input, change nothing else.
//
// Measured before this round, that passed every check on 8 of the 15 samples —
// including `interpunktion`, which scored a flat 1.000 with no punctuation in
// it at all. A suite whose samples can be satisfied by a search-and-replace over
// its own answer key is measuring the answer key.
//
// It is also the test that keeps the checks WIRED IN: unlike the per-function
// tests above, this one scores through RunDeterministicChecks, so a check that
// is dropped from the scorecard shows up here.
func TestTheMinimalReplacementFailsEverySample(t *testing.T) {
	forEachSample(t, func(t *testing.T, name, input, reference string, n SampleNotes) {
		out := input
		for _, f := range n.RequiredFixes {
			out = strings.ReplaceAll(out, f.From, f.To)
		}
		card := RunDeterministicChecks(input, reference, out, n)
		if len(n.RequiredFixes) == 0 {
			if !card.AllPassed {
				t.Errorf("the already-correct sample rejected its own input: %+v", card.Checks)
			}
			return
		}
		if card.AllPassed {
			t.Error("a search-and-replace over the input passed every check")
		}
	})
}
