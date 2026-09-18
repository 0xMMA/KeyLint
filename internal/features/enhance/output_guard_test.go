package enhance

import (
	"strings"
	"testing"
)

// The strings below are the outputs the eval actually recorded, not invented
// shapes. A guard written against imagined failures guards nothing.

const (
	correctInput = "The quarterly figures are final. I have attached the summary and will walk the team through it on Monday."
	chatInput    = "hey, kannst du kurz draufschauen? hab da was gebastelt aber bin mir nicht sicher ob das so passt. kein stress, morgen reicht auch :)"
)

// TestTheAddedNoteIsDropped: all three runs of the first baseline returned the
// already-correct sentence and appended a note about it. In the silent hotkey
// flow that parenthetical is pasted into the user's document.
func TestTheAddedNoteIsDropped(t *testing.T) {
	for name, output := range map[string]string{
		"as recorded": correctInput + "\n\n(No corrections needed—this text is grammatically correct and clear.)",
		"in italics":  correctInput + "\n\n*(No corrections needed—this text is grammatically correct and clear.)*",
		"in brackets": correctInput + " [no changes required]",
		"leading":     "(No corrections needed.)\n\n" + correctInput,
		"same line":   correctInput + " (Everything looks good.)",
	} {
		if got := guardOutput(correctInput, output); got != correctInput {
			t.Errorf("%s: got %q", name, got)
		}
	}
}

// TestTheAuthorsOwnParenthesesSurvive is the other half of that: the guard
// decides by what removing the block leaves behind, not by what the block says,
// so a parenthesis the author wrote is never a candidate.
func TestTheAuthorsOwnParenthesesSurvive(t *testing.T) {
	for name, tc := range map[string]struct{ in, out string }{
		"parenthesis in the text": {
			in:  "the launch (originally friday) moved to monday",
			out: "The launch (originally Friday) moved to Monday.",
		},
		"trailing parenthesis the author wrote": {
			in:  "send it to the team before noon (sarah, tom)",
			out: "Send it to the team before noon (Sarah, Tom).",
		},
		"a note next to a real correction": {
			// The text was genuinely corrected, so removing the block does not
			// leave the input and the guard does not get to guess. The eval
			// still reports this as commentary; the guard is not the check.
			in:  "their going to the meeting",
			out: "They're going to the meeting. (Fixed one contraction.)",
		},
	} {
		if got := guardOutput(tc.in, tc.out); got != tc.out {
			t.Errorf("%s: the guard changed a correction it should not touch: %q", name, got)
		}
	}
}

// TestAReplyIsRefused: two of three runs in the second baseline answered the
// chat message instead of correcting it, one of them keeping 8% of the author's
// words. Pasting that over the user's selection destroys the text it was meant
// to fix, so the guard returns the original.
func TestAReplyIsRefused(t *testing.T) {
	for name, reply := range map[string]string{
		"offers to help": "Hey, klar schau ich mir das an! Allerdings sehe ich hier keinen Text oder Code, den ich überprüfen könnte. Kannst du mir das zeigen, das du gebastelt hast? :)",
		"asks for input": "Kein Problem! Ich bin bereit – schreib einfach hin, was du checken möchtest. :)",
	} {
		if got := guardOutput(chatInput, reply); got != chatInput {
			t.Errorf("%s: a reply was passed through as a correction: %q", name, got)
		}
	}
}

// TestABadCorrectionIsStillACorrection is the false-positive side, and the
// reason the floor sits at 0.4 rather than somewhere comfortable. The run that
// translated a deliberately code-switched clause is wrong — it is the second
// violation in #80 — but it is a correction of this text, and replacing it with
// the input would hide a failure the eval is supposed to report.
func TestABadCorrectionIsStillACorrection(t *testing.T) {
	const mixedInput = "ok so ich hab den branch gemerged, aber die pipeline ist red. läuft wohl an den flaky tests, i will look into it tomorrow morning."
	mistranslated := "Ok, so ich hab den Branch gemerged, aber die Pipeline ist rot. Läuft wohl an den flaky Tests, ich schaue mir das morgen früh an."

	if got := guardOutput(mixedInput, mistranslated); got != mistranslated {
		t.Errorf("a mistranslation was refused as a reply: %q", got)
	}
}

// TestOrdinaryCorrectionsPassUntouched: every sample input that reads like
// something addressed to a reader — a question, a request, a second-person
// message — with the correction the suite expects.
func TestOrdinaryCorrectionsPassUntouched(t *testing.T) {
	for name, tc := range map[string]struct{ in, out string }{
		"a question to a colleague": {
			in:  "here is the report you asked for, can you check it before friday",
			out: "Here is the report you asked for. Can you check it before Friday?",
		},
		"a request": {
			in:  "please send me the rechnung for last month",
			out: "Please send me the invoice for last month.",
		},
		"the chat message, corrected": {
			in:  chatInput,
			out: "Hey, kannst du kurz draufschauen? Hab da was gebastelt, aber bin mir nicht sicher, ob das so passt. Kein Stress, morgen reicht auch :)",
		},
		"already correct": {in: correctInput, out: correctInput},
	} {
		if got := guardOutput(tc.in, tc.out); got != tc.out {
			t.Errorf("%s: the guard changed a correct answer to %q", name, got)
		}
	}
}

// TestShortTextIsNotJudged: below a handful of substantial words one changed
// word swings the ratio past any threshold, so the guard has no opinion.
func TestShortTextIsNotJudged(t *testing.T) {
	for name, tc := range map[string]struct{ in, out string }{
		"two words":  {in: "thanks alot", out: "Thanks a lot."},
		"a greeting": {in: "hey :)", out: "Hey :)"},
	} {
		if got := guardOutput(tc.in, tc.out); got != tc.out {
			t.Errorf("%s: a short correction was refused: %q", name, got)
		}
	}
}

// TestEchoedMarkersAreRemoved: the input is delimited, and a model that wraps
// its answer the same way would otherwise paste the markers into the document.
func TestEchoedMarkersAreRemoved(t *testing.T) {
	wrapped := inputOpen + "\n" + correctInput + "\n" + inputClose
	if got := guardOutput(correctInput, wrapped); got != correctInput {
		t.Errorf("markers survived: %q", got)
	}
	// And text that merely mentions a tag is left alone.
	const htmlish = "Use the <b>bold</b> tag for the heading, not <i>italics</i>."
	if got := guardOutput(htmlish, htmlish); got != htmlish {
		t.Errorf("the guard ate markup that was part of the text: %q", got)
	}
}

// TestTheUserMessageCarriesTheMarkers pins the wrapper itself: the prompt
// promises the model a document between two markers, and the request has to
// keep that promise.
func TestTheUserMessageCarriesTheMarkers(t *testing.T) {
	msg := buildUserMessage("hello")
	if !strings.HasPrefix(msg, inputOpen) || !strings.HasSuffix(msg, inputClose) {
		t.Fatalf("user message = %q", msg)
	}
	if !strings.Contains(msg, "\nhello\n") {
		t.Errorf("the text is not on its own line: %q", msg)
	}
	if !strings.Contains(systemPrompt, inputOpen) || !strings.Contains(systemPrompt, inputClose) {
		t.Error("the system prompt must name the markers the user message uses")
	}
}
