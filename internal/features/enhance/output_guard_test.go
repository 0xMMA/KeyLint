package enhance

import (
	"strings"
	"testing"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
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
		// The four shapes a review found going straight through the first
		// version, which recognised a bracket rather than an addition. The
		// observed failure happened to be parenthesised; nothing says the next
		// one will be.
		"no brackets at all": correctInput + "\n\nI reviewed the text and found nothing to change.",
		"a preamble":         "Here is the corrected text:\n\n" + correctInput,
		"a code fence":       "```\n" + correctInput + "\n```",
		"quoted":             "\"" + correctInput + "\"",
	} {
		if got, _ := guardOutput(correctInput, output); got != correctInput {
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
		if got, _ := guardOutput(tc.in, tc.out); got != tc.out {
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
		if got, _ := guardOutput(chatInput, reply); got != chatInput {
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

	if got, _ := guardOutput(mixedInput, mistranslated); got != mistranslated {
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
		if got, _ := guardOutput(tc.in, tc.out); got != tc.out {
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
		if got, _ := guardOutput(tc.in, tc.out); got != tc.out {
			t.Errorf("%s: a short correction was refused: %q", name, got)
		}
	}
}

// TestEchoedMarkersAreRemoved: the input is delimited, and a model that wraps
// its answer the same way would otherwise paste the markers into the document.
func TestEchoedMarkersAreRemoved(t *testing.T) {
	wrapped := inputOpen + "\n" + correctInput + "\n" + inputClose
	if got, _ := guardOutput(correctInput, wrapped); got != correctInput {
		t.Errorf("markers survived: %q", got)
	}
	// And text that merely mentions a tag is left alone.
	const htmlish = "Use the <b>bold</b> tag for the heading, not <i>italics</i>."
	if got, _ := guardOutput(htmlish, htmlish); got != htmlish {
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

// TestHeavilyCorrectedTextSurvives is the false-positive case the first guard
// failed. It measured retention as word identity, and a correction changes the
// middle of a word: typing umlauts as ue/oe/ae is ordinary German input, and on
// those texts the guard threw the correction away and silently returned the
// user's typos. The suite's own de-umlaute-ascii sample scored 0.462 against a
// floor of 0.4 — the guard was sitting next to the normal case, not above it.
func TestHeavilyCorrectedTextSurvives(t *testing.T) {
	for name, tc := range map[string]struct{ in, out string }{
		"ascii umlauts": {
			in:  "hallo, koennten wir den termin fuer die pruefung verschieben? ich muesste vorher noch die groesse der raeume klaeren",
			out: "Hallo, könnten wir den Termin für die Prüfung verschieben? Ich müsste vorher noch die Größe der Räume klären.",
		},
		"the suite's own umlaut sample": {
			in:  "fuer die naechste woche brauche ich noch die groesse der dateien, ausserdem waere es gut wenn wir die pruefung vorziehen koennten.",
			out: "Für die nächste Woche brauche ich noch die Größe der Dateien. Außerdem wäre es gut, wenn wir die Prüfung vorziehen könnten.",
		},
		"dense typos": {
			in:  "the enviroment configuraton was seperate from the develepment pipline, wich ment the deploment recieved diferent paramaters",
			out: "The environment configuration was separate from the development pipeline, which meant the deployment received different parameters.",
		},
		"german compound spacing": {
			in:  "bitte schick mir die projekt daten und die kunden liste bis freitag, die termin planung machen wir dann in der team sitzung",
			out: "Bitte schick mir die Projektdaten und die Kundenliste bis Freitag, die Terminplanung machen wir dann in der Teamsitzung.",
		},
		"identifiers and numbers": {
			in:  "invoice 4711 for customer ACME-2024 is overdue, pls chase it up before the 15th, ref PO-98231",
			out: "Invoice 4711 for customer ACME-2024 is overdue. Please chase it up before the 15th, ref PO-98231.",
		},
	} {
		if got, action := guardOutput(tc.in, tc.out); got != tc.out {
			t.Errorf("%s: a correction was refused (%s): %q", name, action, got)
		}
	}
}

// TestTextMentioningTheMarkersIsNotCut: the first version searched for the
// markers anywhere in the output and cut there, so a sentence that merely
// mentions one — which anyone working on KeyLint might write — came back with
// everything before it deleted, and the truncation was pasted over the user's
// selection.
func TestTextMentioningTheMarkersIsNotCut(t *testing.T) {
	const (
		in  = "in <text-to-correct> we wrap the users clipboard text before we send it to the model, thats the whole trick"
		out = "In <text-to-correct> we wrap the user's clipboard text before we send it to the model; that's the whole trick."
	)
	if got, action := guardOutput(in, out); got != out {
		t.Errorf("the guard cut the author's text (%s): %q", action, got)
	}
}

// TestTheGuardIsWiredIntoEnhance: every guard test above calls guardOutput
// directly, so all of them passed with the guard removed from the one path the
// GUI and the CLI share. This is the test that fails when that happens.
func TestTheGuardIsWiredIntoEnhance(t *testing.T) {
	cfg := settings.Default()
	cfg.ActiveProvider = "claude"
	const reply = "Kein Problem! Ich bin bereit – schreib einfach hin, was du checken möchtest. :)"
	svc, _ := newTestService(t, cfg, map[string]string{"claude": "sk-test"})
	svc.newClient = func(string, llm.Config) (llm.Client, error) {
		return &fakeClient{reply: reply}, nil
	}

	got, err := svc.Enhance(chatInput)
	if err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if got != chatInput {
		t.Errorf("Enhance returned the model's reply instead of the author's text: %q", got)
	}
	if svc.lastGuardAction != guardRefused {
		t.Errorf("guard action = %q, want %q", svc.lastGuardAction, guardRefused)
	}
}
