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
			system, user, err := buildPromptV3(form, "casual", "close", "keep it short", "the input")
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
			for _, want := range []string{"Communication style: casual", "Relationship level: close", "Additional instructions: keep it short"} {
				if !strings.Contains(system, want) {
					t.Errorf("the %s prompt lacks the user's style: %q", form, want)
				}
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
		"quoted replies",        // the whole input is source material
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

// TestAnExplicitChoiceWinsOverTheVoiceDefaults pins Michael's decision of
// 2026-10-11: without a choice the author's language, code-switching and tone
// stay; a style, relationship or instruction the user chose wins over them;
// content never changes. The UI always sends professional/professional, so
// that pair must read as the default, not as a choice.
func TestAnExplicitChoiceWinsOverTheVoiceDefaults(t *testing.T) {
	for _, rule := range []string{
		"by default, keep the author's voice",
		"NEVER translate on your own",
		"an explicit choice wins over the voice defaults",
		`"professional" style with "professional" relationship is the default, not a choice`,
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

// TestFixedFormsKeepTheirShape pins Michael's decision of 2026-10-11: an email
// always has subject, greeting, bold headers and sign-off, also for three
// casual sentences; memo, wiki and presentation keep their shapes too. Chat is
// the form for a short result. A rule that lets a short input drop the shape
// contradicts that — and the eval's structure check, which wants headers.
func TestFixedFormsKeepTheirShape(t *testing.T) {
	for _, contradiction := range []string{"as much structure as the content needs", "a short message gets a short result", "short input makes a short"} {
		for form := range promptV3Forms {
			system, _, _ := buildPromptV3(form, "professional", "professional", "", "x")
			if strings.Contains(system, contradiction) {
				t.Errorf("the %s prompt says %q", form, contradiction)
			}
		}
	}
	if !strings.Contains(promptV3Core, "always take their full shape, however short the input") {
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
		system, _, _ := buildPromptV3(form, "professional", "professional", "", "x")
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
		system, user, _, err := buildDocTypePrompt("email", 2, tc.style, tc.relationship, tc.custom, "draft")
		if err != nil {
			t.Fatalf("buildDocTypePrompt: %v", err)
		}
		sum := sha256.Sum256([]byte(system))
		if got := hex.EncodeToString(sum[:]); got != tc.want || user != "draft" {
			t.Errorf("v2's prompt for %s/%s changed (sha256 %s); the measured baseline no longer describes it", tc.style, tc.relationship, got)
		}
	}
}
