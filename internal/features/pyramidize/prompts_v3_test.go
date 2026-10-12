package pyramidize

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// forms is every form v3 writes, under the document type that selects it.
var forms = []string{"email", "chat", "memo", "wiki", "powerpoint"}

// TestEveryFormHasASection: the five forms of ADR-003, no more, no fewer — a
// form the UI offers without a section would fail at run time.
func TestEveryFormHasASection(t *testing.T) {
	var got []string
	for form := range promptV3Forms {
		got = append(got, form)
		if !isValidDocType(form) {
			t.Errorf("v3 has a section for %q, which is not a valid document type", form)
		}
	}
	sort.Strings(got)
	want := append([]string(nil), forms...)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("v3 sections = %v, want %v", got, want)
	}
}

// TestV3PromptsAreTheCorePlusTheirForm: each form's prompt carries the shared
// core, its own section and the output format — and no other form's section,
// which would hand the model two contradicting shapes.
func TestV3PromptsAreTheCorePlusTheirForm(t *testing.T) {
	for _, form := range forms {
		t.Run(form, func(t *testing.T) {
			system, user, err := buildPromptV3(form, "keep it short", "the input")
			if err != nil {
				t.Fatalf("buildPromptV3: %v", err)
			}
			if user != "the input" {
				t.Errorf("user message = %q, want the input text unchanged", user)
			}
			for name, part := range map[string]string{"core": promptV3Core, "own section": promptV3Forms[form], "output format": promptV3Output} {
				if !strings.Contains(system, part) {
					t.Errorf("the %s prompt lacks its %s", form, name)
				}
			}
			for other, section := range promptV3Forms {
				if other != form && strings.Contains(system, section) {
					t.Errorf("the %s prompt carries the %s section too", form, other)
				}
			}
			if !strings.Contains(system, "<user_instructions>\nkeep it short\n</user_instructions>") {
				t.Errorf("the %s prompt lacks the user's instructions", form)
			}
		})
	}
}

// TestTheCoreCarriesV2sProvenRules: v3 restructures v2-email, it does not
// reinvent it. These are the rules v2 shipped with and was measured on; each
// one dropping out of the core is a change the v3-vs-v2 measurement would have
// to explain.
func TestTheCoreCarriesV2sProvenRules(t *testing.T) {
	for _, rule := range []string{
		"Pyramid Principle",
		// v2's "quoted replies … are source material" is NOT carried: mail
		// history is context only since 2026-10-11, see
		// TestMailHistoryIsContextNeverOutput.
		"NEVER a process label", // content-statement headers
		"mutually exclusive, collectively exhaustive",
		"decisions → actions → context",
		"NEVER translate",
		"code-switching",
		"NEVER increase formality or change perspective",
		"NEVER add conclusions, interpretations",
		// What v3 adds, from quality-status.md's persistent behaviours.
		`"no action needed"`, // no editorial status claims
	} {
		if !strings.Contains(promptV3Core, rule) {
			t.Errorf("the v3 core lacks %q", rule)
		}
	}
}

// TestMailHistoryIsContextNeverOutput pins Michael's decision of 2026-10-11
// (ADR-003, addendum): mail history in the selection — quoted replies,
// forwarded mails, the thread — is context. The output never reproduces it;
// facts from it may be referred to and must not be contradicted. Fidelity
// covers the author's own message. The prompt states the rule, not the
// reasons for it.
func TestMailHistoryIsContextNeverOutput(t *testing.T) {
	for _, rule := range []string{
		"quoted replies, forwarded messages, earlier messages in the thread",
		"mail history is context only",
		"never reproduce it: no recap, no history section, no quoted originals",
		"never contradict it",
		"in the author's own message appears in the output",
	} {
		if !strings.Contains(promptV3Core, rule) {
			t.Errorf("the v3 core lacks %q", rule)
		}
	}
	// v3's first rule did the opposite: it kept the history, at the end.
	for _, gone := range []string{"they move to the end, they do not drop out", "is source material", "cover the whole input"} {
		if strings.Contains(promptV3Core, gone) {
			t.Errorf("the v3 core still says %q", gone)
		}
	}
	// Language, asks, greeting and sign-off are the author's, not those of a
	// quoted mail.
	if !strings.Contains(promptV3Core, "write in the language of the author's message") {
		t.Error("the v3 core takes the reply's language from the whole input, quoted mails included")
	}
	if !strings.Contains(promptV3Forms["email"], "when the author asks something of someone") {
		t.Error("the email subject takes its @person asks from the whole input, quoted mails included")
	}
	// Every rule that says what the output keeps or is written in names the
	// author's message, not "the input", which would take in the history.
	// <fidelity>'s "information not in the input" stays: referring to the
	// history is allowed.
	for _, form := range forms {
		system, _, _ := buildPromptV3(form, "", "x")
		for _, gone := range []string{
			"asks stay as in the input",
			"courtesy phrases the input does not have",
			"language of the input",
			"source references and links from the input",
			"numbers and facts from the input",
		} {
			if strings.Contains(system, gone) {
				t.Errorf("the %s prompt still says %q", form, gone)
			}
		}
	}
	for part, phrase := range map[string]string{
		promptV3Core:                "asks stay as in the author's message",
		promptV3Output:              "language of the author's message (ISO 639-1)",
		promptV3Forms["memo"]:       "labels in the language of the author's message",
		promptV3Forms["wiki"]:       "keep source references and links from the author's message",
		promptV3Forms["powerpoint"]: "specific numbers and facts from the author's message",
	} {
		if !strings.Contains(part, phrase) {
			t.Errorf("v3 lacks %q", phrase)
		}
	}
	if !strings.Contains(promptV3Core, "information not in the input") {
		t.Error("<fidelity> must keep \"information not in the input\": a reference to the history is not added information")
	}
	for form, phrase := range map[string]string{
		"email": "the author's greeting, exactly as written",
		"chat":  "only if the author's message has them",
	} {
		if !strings.Contains(promptV3Forms[form], phrase) {
			t.Errorf("the %s section lacks %q", form, phrase)
		}
	}
}

// TestAnExplicitChoiceWinsOverTheVoiceDefaults pins Michael's decision of
// 2026-10-11: without a choice the author's language, code-switching and tone
// stay; an instruction the user gave wins over them; content never changes.
func TestAnExplicitChoiceWinsOverTheVoiceDefaults(t *testing.T) {
	for _, rule := range []string{
		"by default, keep the author's voice",
		"NEVER translate on your own",
		"an explicit choice wins over the voice defaults",
		`"auf Englisch" means write in English`,
		`"formeller" means raise the formality`,
		"a choice changes how the text reads, never what it says",
		"fidelity holds whatever the user chooses",
	} {
		if !strings.Contains(promptV3Core, rule) {
			t.Errorf("the v3 core lacks %q", rule)
		}
	}
}

// userChoices is the core's <user_choices> section.
func userChoices(t *testing.T) string {
	t.Helper()
	// The opening tag on a line of its own: <fidelity> names the section too.
	_, rest, ok := strings.Cut(promptV3Core, "\n<user_choices>\n")
	section, _, closed := strings.Cut(rest, "\n</user_choices>")
	if !ok || !closed {
		t.Fatal("the v3 core has no <user_choices> section")
	}
	return section
}

// TestUserChoicesAreTheUsersInstructionsOnly pins the choice page decision of
// 2026-10-12 (roadmap E3, ADR-003 addendum): the form is the user's only pick
// besides their own instructions. There is no style, relationship, tone or
// audience control any more, so <user_choices> speaks of instructions only,
// and no prompt carries a style or relationship value.
func TestUserChoicesAreTheUsersInstructionsOnly(t *testing.T) {
	section := strings.ToLower(userChoices(t))
	for _, gone := range []string{"style", "relationship", "professional", "authority", "public", "casual", "close", "audience", "tone"} {
		if regexp.MustCompile(`\b` + gone + `\b`).MatchString(section) {
			t.Errorf("<user_choices> still mentions %q", gone)
		}
	}
	for _, rule := range []string{
		"the user's own words",
		"over the voice defaults and the form's defaults",
		`"auf englisch" means write in english`,
		`"formeller" means raise the formality`,
		"a choice changes how the text reads, never what it says",
	} {
		if !strings.Contains(section, rule) {
			t.Errorf("<user_choices> lacks %q", rule)
		}
	}
	for _, form := range forms {
		for _, instructions := range []string{"", "auf Englisch"} {
			system, _, _ := buildPromptV3(form, instructions, "x")
			for _, gone := range []string{"<style_injection>", "Communication style", "Relationship level"} {
				if strings.Contains(system, gone) {
					t.Errorf("the %s prompt still carries %q", form, gone)
				}
			}
		}
	}
}

// TestInstructionsReachTheV3Prompt: what the user types under "Instructions"
// is in the prompt, in the block <user_choices> names, and a prompt without
// instructions carries no empty block.
func TestInstructionsReachTheV3Prompt(t *testing.T) {
	for _, form := range forms {
		system, user, _ := buildPromptV3(form, "  for the whole department  ", "the text")
		if !strings.Contains(system, "<user_instructions>\nfor the whole department\n</user_instructions>") {
			t.Errorf("the %s prompt lacks the instructions block", form)
		}
		if user != "the text" {
			t.Errorf("the %s user message = %q, want the text", form, user)
		}
		if strings.Contains(user, "department") {
			t.Errorf("the %s instructions leaked into the user message", form)
		}
		bare, _, _ := buildPromptV3(form, "  ", "the text")
		if strings.Contains(bare, "<user_instructions>\n") {
			t.Errorf("the %s prompt carries an instructions block without instructions", form)
		}
	}
}

// TestFixedFormsKeepTheirShape pins Michael's decision of 2026-10-11: an email
// always has subject, greeting, bold headers and sign-off, also for three
// casual sentences; memo, wiki and presentation keep their shapes too. Chat is
// the form for a short result. A rule that lets a short input drop the shape
// contradicts that — and the eval's structure check, which wants headers.
func TestFixedFormsKeepTheirShape(t *testing.T) {
	for _, contradiction := range []string{"as much structure as the content needs", "a short message gets a short result", "short input makes a short"} {
		for form := range promptV3Forms {
			system, _, _ := buildPromptV3(form, "", "x")
			if strings.Contains(system, contradiction) {
				t.Errorf("the %s prompt says %q", form, contradiction)
			}
		}
	}
	if !strings.Contains(promptV3Core, "always take their full shape, however short the author's message") {
		t.Error("the core does not say the fixed forms keep their shape")
	}
	if !strings.Contains(promptV3Forms["email"], "always this shape, also for a three-sentence casual mail") {
		t.Error("the email section does not say every email takes the full shape")
	}
	// A minimum invites padding: the presentation example itself has two bullets.
	if !strings.Contains(promptV3Forms["powerpoint"], "at most 5 bullet points") || strings.Contains(promptV3Forms["powerpoint"], "3–5") {
		t.Error("the presentation section must cap bullets, not require a minimum")
	}
}

// TestExamplesKeepTheAuthorsPerson: an example whose input says "we" keeps
// "we" — the core's voice rule, which the first memo and wiki examples broke.
func TestExamplesKeepTheAuthorsPerson(t *testing.T) {
	we := regexp.MustCompile(`(?i)\bwe('re)?\b`)
	input := regexp.MustCompile(`(?m)^input: (".*")$`)
	for form, section := range promptV3Forms {
		inputs := input.FindAllStringSubmatch(section, -1)
		outputs := exampleOutput.FindAllStringSubmatch(section, -1)
		for i := range inputs {
			if i < len(outputs) && we.MatchString(inputs[i][1]) && !we.MatchString(outputs[i][1]) {
				t.Errorf("the %s example drops the author's \"we\"", form)
			}
		}
	}
}

// TestV3AsksForNoQualityFields: nothing in v3 may ask for a self-assessment.
// There is no refine pass to act on one, and an invented score is noise in the
// result (ADR-002 decision 6).
func TestV3AsksForNoQualityFields(t *testing.T) {
	for _, form := range forms {
		system, _, _ := buildPromptV3(form, "", "x")
		for _, marker := range []string{"qualityScore", "qualityFlags", "self_evaluation", "headers\":"} {
			if strings.Contains(system, marker) {
				t.Errorf("the %s prompt mentions %q", form, marker)
			}
		}
	}
}

// exampleOutput finds every example reply in a prompt.
var exampleOutput = regexp.MustCompile(`(?m)^output: (\{.*\})$`)

// TestV3ExamplesAreValidReplies: each example reply is valid JSON in exactly
// the shape the output format asks for, with a document in its form's shape.
// An example that contradicts the rules teaches the contradiction — v2's
// "Keine Aktion erforderlich" subject was one.
func TestV3ExamplesAreValidReplies(t *testing.T) {
	firstLine := map[string]*regexp.Regexp{
		"email":      regexp.MustCompile(`^[^\n]+ \| [^\n]+$`), // pipe-separated subject
		"memo":       regexp.MustCompile(`^\*\*[^*]+:\*\* .+ \| \*\*[^*]+:\*\* .+ \| \*\*[^*]+:\*\* .+$`),
		"wiki":       regexp.MustCompile(`^# .+$`),
		"powerpoint": regexp.MustCompile(`^# Slide 1: .+$`),
	}
	for _, form := range forms {
		t.Run(form, func(t *testing.T) {
			matches := exampleOutput.FindAllStringSubmatch(promptV3Forms[form], -1)
			if len(matches) == 0 {
				t.Fatalf("the %s section has no example", form)
			}
			for _, m := range matches {
				var reply map[string]any
				if err := json.Unmarshal([]byte(m[1]), &reply); err != nil {
					t.Fatalf("example is not valid JSON: %v\n%s", err, m[1])
				}
				if len(reply) != 2 || reply["fullDocument"] == nil || reply["language"] == nil {
					t.Errorf("example keys = %v, want exactly fullDocument and language", keys(reply))
				}
				doc, _ := reply["fullDocument"].(string)
				first, _, _ := strings.Cut(doc, "\n")
				if re, ok := firstLine[form]; ok && !re.MatchString(first) {
					t.Errorf("example's first line %q is not the %s form's", first, form)
				}
				if form == "chat" && (strings.Contains(doc, "**") || strings.Contains(doc, "\n#")) {
					t.Errorf("the chat example has headers: %q", doc)
				}
			}
		})
	}
}

func keys(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestV2IsKeptVerbatim: v2 stays selectable for the v3-vs-v2 measurement, and
// it is only a baseline while it is the prompt that was measured. The hashes
// were computed from main's prompts_email_v2.go (c10706a), built on its own —
// not from this package's code, which would pin whatever it says today.
func TestV2IsKeptVerbatim(t *testing.T) {
	for _, tc := range []struct{ style, relationship, custom, want string }{
		{"professional", "professional", "", "e0613093c5e862f9f93deaf3caf0133e7200a53711d768dade509d438953eb04"},
		{"casual", "close", "keep it short", "411c0a10ac34e4725f55e9fe85b9f80c80808c03aa9c332b9f9c554f6760ae78"},
	} {
		system, user := buildEmailPromptV2(tc.style, tc.relationship, tc.custom, "draft")
		sum := sha256.Sum256([]byte(system))
		if got := hex.EncodeToString(sum[:]); got != tc.want || user != "draft" {
			t.Errorf("v2's prompt for %s/%s changed (sha256 %s); the measured baseline no longer describes it", tc.style, tc.relationship, got)
		}
	}
	// What a run sends v2 since the choice page: no style or relationship to
	// pass on, so v2 gets the professional/professional it was measured with.
	system, user, _, err := buildDocTypePrompt("email", 2, "", "draft")
	if err != nil {
		t.Fatalf("buildDocTypePrompt: %v", err)
	}
	sum := sha256.Sum256([]byte(system))
	if got := hex.EncodeToString(sum[:]); got != "e0613093c5e862f9f93deaf3caf0133e7200a53711d768dade509d438953eb04" || user != "draft" {
		t.Errorf("a v2 run no longer sends the measured professional/professional prompt (sha256 %s)", got)
	}
}
