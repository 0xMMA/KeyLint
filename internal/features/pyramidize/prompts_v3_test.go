package pyramidize

import (
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
		"never padding",      // a short message stays short
	} {
		if !strings.Contains(promptV3Core, rule) {
			t.Errorf("the v3 core lacks %q", rule)
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
// it is only a baseline while it is the prompt that was measured.
func TestV2IsKeptVerbatim(t *testing.T) {
	system, user, _, err := buildDocTypePrompt("email", 2, "professional", "professional", "", "draft")
	if err != nil {
		t.Fatalf("buildDocTypePrompt: %v", err)
	}
	if system != emailSystemBaseV2+"\n\n"+styleInjection("professional", "professional", "") || user != "draft" {
		t.Error("v2's prompt changed; the measured baseline no longer describes it")
	}
}
