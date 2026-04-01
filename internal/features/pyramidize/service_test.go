package pyramidize

import (
	"strings"
	"testing"
)

// --- isValidDocType ---

func TestIsValidDocType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"email", true},
		{"wiki", true},
		{"memo", true},
		{"powerpoint", true},
		{"", false},
		{"auto", false},
		{"EMAIL", false},  // case-sensitive
		{"pdf", false},
		{"blog", false},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := isValidDocType(tc.input); got != tc.want {
				t.Errorf("isValidDocType(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// --- stripFences ---

func TestStripFences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no fences",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "json fences",
			input: "```json\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "bare fences",
			input: "```\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON uppercase fences",
			input: "```JSON\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "leading and trailing whitespace",
			input: "   \n{\"key\": \"value\"}\n   ",
			want:  `{"key": "value"}`,
		},
		{
			name:  "fences with extra whitespace",
			input: "```json\n\n{\"key\": \"value\"}\n\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "no trailing fence",
			input: "```json\n{\"key\": \"value\"}",
			want:  `{"key": "value"}`,
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only whitespace",
			input: "   \n  \t  ",
			want:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripFences(tc.input)
			if got != tc.want {
				t.Errorf("stripFences(%q)\ngot:  %q\nwant: %q", tc.input, got, tc.want)
			}
		})
	}
}

// --- unmarshalRobust ---

func TestUnmarshalRobust(t *testing.T) {
	t.Run("plain JSON", func(t *testing.T) {
		var r detectResult
		err := unmarshalRobust(`{"type":"EMAIL","language":"en","confidence":0.9}`, &r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Type != "EMAIL" || r.Language != "en" || r.Confidence != 0.9 {
			t.Errorf("unexpected result: %+v", r)
		}
	})

	t.Run("fenced JSON", func(t *testing.T) {
		var r detectResult
		err := unmarshalRobust("```json\n{\"type\":\"WIKI\",\"language\":\"de\",\"confidence\":0.8}\n```", &r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Type != "WIKI" || r.Language != "de" {
			t.Errorf("unexpected result: %+v", r)
		}
	})

	t.Run("whitespace around JSON", func(t *testing.T) {
		var r detectResult
		err := unmarshalRobust("  \n{\"type\":\"MEMO\",\"language\":\"fr\",\"confidence\":0.7}\n  ", &r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Type != "MEMO" {
			t.Errorf("unexpected type: %q", r.Type)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		var r detectResult
		err := unmarshalRobust("not json at all", &r)
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("fenced invalid JSON returns error", func(t *testing.T) {
		var r detectResult
		err := unmarshalRobust("```json\nnot json\n```", &r)
		if err == nil {
			t.Error("expected error for invalid fenced JSON, got nil")
		}
	})

	t.Run("foundation result", func(t *testing.T) {
		var r foundationResult
		raw := `{"fullDocument":"Subject | Detail\n\nBody","headers":["H1","H2"],"language":"en","qualityScore":0.85,"qualityFlags":[]}`
		if err := unmarshalRobust(raw, &r); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.QualityScore != 0.85 {
			t.Errorf("expected qualityScore 0.85, got %v", r.QualityScore)
		}
		if len(r.Headers) != 2 {
			t.Errorf("expected 2 headers, got %d", len(r.Headers))
		}
	})
}

// --- repairJSONStrings ---

func TestRepairJSONStrings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no repair needed",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "already escaped quote",
			input: `{"key": "say \"hi\""}`,
			want:  `{"key": "say \"hi\""}`,
		},
		{
			name:  "unescaped quote in value — not followed by structural char",
			input: `{"fullDocument": "he said "hello" and left"}`,
			want:  `{"fullDocument": "he said \"hello\" and left"}`,
		},
		{
			name:  "real-world LLM output: German email with unescaped close quote",
			// The closing " of a quoted phrase is followed by a space then a word —
			// not a structural char. The repair should escape it.
			input: `{"fullDocument": "next Steps Projekt "Qlik Sense" abgeschlossen","qualityScore":0.9,"qualityFlags":[]}`,
			want:  `{"fullDocument": "next Steps Projekt \"Qlik Sense\" abgeschlossen","qualityScore":0.9,"qualityFlags":[]}`,
		},
		{
			name:  "multiple unescaped quotes in one value",
			input: `{"text": "word "one" and "two" here"}`,
			want:  `{"text": "word \"one\" and \"two\" here"}`,
		},
		{
			name:  "quote at end of value — structural comma follows",
			input: `{"a": "val", "b": "other"}`,
			want:  `{"a": "val", "b": "other"}`,
		},
		{
			name:  "empty value",
			input: `{"k": ""}`,
			want:  `{"k": ""}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := repairJSONStrings(tc.input)
			if got != tc.want {
				t.Errorf("repairJSONStrings:\ninput: %s\ngot:   %s\nwant:  %s", tc.input, got, tc.want)
			}
		})
	}
}

func TestUnmarshalRobustRepairs(t *testing.T) {
	t.Run("unescaped quote in fullDocument value", func(t *testing.T) {
		// Simulates the real bug: Claude returns JSON where the document text
		// contains an unescaped " char (e.g. „Qlik Sense" with ASCII closing quote).
		raw := "```json\n{\"fullDocument\": \"next Steps Projekt \\u201eQlik Sense\\\" abgeschlossen\",\"headers\":[],\"language\":\"de\",\"qualityScore\":0.9,\"qualityFlags\":[]}\n```"
		// Use a simpler direct case that matches the actual failure scenario:
		raw2 := `{"fullDocument": "Steps "Qlik Sense" done","headers":[],"language":"de","qualityScore":0.85,"qualityFlags":[]}`
		var r foundationResult
		if err := unmarshalRobust(raw2, &r); err != nil {
			t.Fatalf("unmarshalRobust failed on unescaped quote input: %v", err)
		}
		if r.Language != "de" {
			t.Errorf("expected language 'de', got %q", r.Language)
		}
		if r.QualityScore != 0.85 {
			t.Errorf("expected qualityScore 0.85, got %v", r.QualityScore)
		}
		_ = raw
	})

	t.Run("trailing content after JSON object", func(t *testing.T) {
		// json.Decoder handles trailing content; json.Unmarshal does not.
		var r detectResult
		raw := `{"type":"EMAIL","language":"en","confidence":0.95}  extra stuff`
		if err := unmarshalRobust(raw, &r); err != nil {
			t.Fatalf("unmarshalRobust failed on trailing content: %v", err)
		}
		if r.Type != "EMAIL" {
			t.Errorf("expected type EMAIL, got %q", r.Type)
		}
	})
}

// --- LookupDocType ---

func TestLookupDocType(t *testing.T) {
	tests := []struct {
		sourceApp string
		want      string
	}{
		{"Microsoft Outlook", "email"},
		{"Thunderbird", "email"},
		{"Apple Mail", "email"},
		{"Microsoft Teams", "email"},
		{"Slack", "email"},
		{"Visual Studio Code", "memo"},
		{"VSCode - main.go", "memo"},
		{"Notepad++", "memo"},
		{"Sublime Text", "memo"},
		{"Microsoft Word", "memo"},
		{"Google Docs", "memo"},
		{"Microsoft PowerPoint", "powerpoint"},
		{"Keynote", "powerpoint"},
		{"LibreOffice Impress", "powerpoint"},
		{"Firefox - GitHub", ""},   // unknown
		{"Terminal", ""},           // unknown
		{"", ""},                   // empty
		{"OUTLOOK 365", "email"},   // case-insensitive
		{"slack - #general", "email"},
	}
	for _, tc := range tests {
		t.Run(tc.sourceApp, func(t *testing.T) {
			got := LookupDocType(tc.sourceApp)
			if got != tc.want {
				t.Errorf("LookupDocType(%q) = %q, want %q", tc.sourceApp, got, tc.want)
			}
		})
	}
}

// --- buildRefinePrompt ---

func TestBuildRefinePrompt(t *testing.T) {
	originalText := "meeting with alice on tuesday, budget approved for $50k"
	failedOutput := "Meeting Notes\n\nBudget discussed."

	t.Run("MECE_VIOLATION flag present in prompt", func(t *testing.T) {
		sys, user := buildRefinePrompt(originalText, failedOutput, []string{"MECE_VIOLATION"})
		if !strings.Contains(sys, "MECE_VIOLATION") {
			t.Error("system prompt does not mention MECE_VIOLATION")
		}
		if !strings.Contains(sys, "mutually exclusive") {
			t.Error("system prompt does not include MECE remediation guidance")
		}
		if !strings.Contains(user, originalText) {
			t.Error("user message does not contain original text")
		}
		if !strings.Contains(user, failedOutput) {
			t.Error("user message does not contain failed output")
		}
	})

	t.Run("INFO_LOSS flag includes hard constraint language", func(t *testing.T) {
		sys, _ := buildRefinePrompt(originalText, failedOutput, []string{"INFO_LOSS"})
		if !strings.Contains(sys, "INFO_LOSS") {
			t.Error("system prompt does not mention INFO_LOSS")
		}
		if !strings.Contains(sys, "hard constraint") {
			t.Error("system prompt does not include hard constraint language for INFO_LOSS")
		}
	})

	t.Run("STYLE_MISMATCH flag", func(t *testing.T) {
		sys, _ := buildRefinePrompt(originalText, failedOutput, []string{"STYLE_MISMATCH"})
		if !strings.Contains(sys, "STYLE_MISMATCH") {
			t.Error("system prompt does not mention STYLE_MISMATCH")
		}
	})

	t.Run("SUBJECT_FORMAT flag", func(t *testing.T) {
		sys, _ := buildRefinePrompt(originalText, failedOutput, []string{"SUBJECT_FORMAT"})
		if !strings.Contains(sys, "SUBJECT_FORMAT") {
			t.Error("system prompt does not mention SUBJECT_FORMAT")
		}
	})

	t.Run("multiple flags", func(t *testing.T) {
		flags := []string{"MECE_VIOLATION", "INFO_LOSS", "STYLE_MISMATCH"}
		sys, _ := buildRefinePrompt(originalText, failedOutput, flags)
		for _, f := range flags {
			if !strings.Contains(sys, f) {
				t.Errorf("system prompt does not mention flag %q", f)
			}
		}
	})

	t.Run("unknown flag included verbatim", func(t *testing.T) {
		sys, _ := buildRefinePrompt(originalText, failedOutput, []string{"CUSTOM_FLAG_XYZ"})
		if !strings.Contains(sys, "CUSTOM_FLAG_XYZ") {
			t.Error("system prompt does not include unknown flag verbatim")
		}
	})

	t.Run("empty flags still builds valid prompt", func(t *testing.T) {
		sys, user := buildRefinePrompt(originalText, failedOutput, []string{})
		if sys == "" {
			t.Error("system prompt should not be empty")
		}
		if user == "" {
			t.Error("user message should not be empty")
		}
	})

	t.Run("original text appears as hard constraint in user message", func(t *testing.T) {
		_, user := buildRefinePrompt(originalText, failedOutput, []string{"INFO_LOSS"})
		if !strings.Contains(user, "hard constraint") {
			t.Error("user message should label original text as hard constraint")
		}
	})
}

// --- buildDocTypePrompt dispatch ---

func TestBuildDocTypePromptDispatch(t *testing.T) {
	types := []string{"email", "wiki", "memo", "powerpoint", "unknown"}
	for _, dt := range types {
		t.Run(dt, func(t *testing.T) {
			sys, user := buildDocTypePrompt(dt, 0, "professional", "professional", "", "some text")
			if sys == "" {
				t.Error("system prompt should not be empty")
			}
			if user != "some text" {
				t.Errorf("user message should be the input text, got %q", user)
			}
		})
	}
}

// --- email prompt variant dispatch ---

func TestEmailPromptVariants(t *testing.T) {
	tests := []struct {
		variant     int
		wantContain string
		desc        string
	}{
		{0, "emailSystemBaseV2", "variant 0 (latest) resolves to v2"},
		{1, "selfQABlock", "variant 1 uses v1 with selfQA"},
		{2, "emailSystemBaseV2", "variant 2 uses v2"},
		{99, "selfQABlock", "unknown variant falls back to v1"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			sys, _ := buildEmailPrompt(tt.variant, "professional", "professional", "", "test input")
			if sys == "" {
				t.Fatal("system prompt should not be empty")
			}
			// v1 includes selfQABlock (contains "self_evaluation"), v2 does not
			hasSelfQA := containsSubstring(sys, "self_evaluation")
			if tt.variant == 1 || tt.variant == 99 {
				if !hasSelfQA {
					t.Error("v1 prompt should contain selfQA block")
				}
			}
			if tt.variant == 0 || tt.variant == 2 {
				if hasSelfQA {
					t.Error("v2 prompt should NOT contain selfQA block")
				}
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && strings.Contains(s, substr)
}

// --- isValidDocType edge cases ---

func TestIsValidDocTypeEdgeCases(t *testing.T) {
	// Verify that auto is explicitly not a valid doc type
	if isValidDocType("auto") {
		t.Error("auto should not be a valid doc type for isValidDocType")
	}
	// Boundary: powerpoint is valid, ppt is not
	if isValidDocType("ppt") {
		t.Error("ppt should not be valid — only powerpoint")
	}
}
