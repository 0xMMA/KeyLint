package pyramidize

import (
	"testing"
)

// --- isValidDocType ---

func TestIsValidDocType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"email", true},
		{"chat", true},
		{"wiki", true},
		{"memo", true},
		{"powerpoint", true},
		{"", false},
		{"auto", false},
		{"EMAIL", false},        // case-sensitive: callers normalise first
		{"presentation", false}, // an alias, mapped by normalizeDocType
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

// probeResult is a small JSON shape for the parser tests; the parser is
// generic and has no reply type of its own to test against.
type probeResult struct {
	Type       string  `json:"type"`
	Language   string  `json:"language"`
	Confidence float64 `json:"confidence"`
}

// headedResult carries an array, for the array-element repair.
type headedResult struct {
	FullDocument string   `json:"fullDocument"`
	Headers      []string `json:"headers"`
	Language     string   `json:"language"`
}

func TestUnmarshalRobust(t *testing.T) {
	t.Run("plain JSON", func(t *testing.T) {
		var r probeResult
		err := unmarshalRobust(`{"type":"EMAIL","language":"en","confidence":0.9}`, &r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Type != "EMAIL" || r.Language != "en" || r.Confidence != 0.9 {
			t.Errorf("unexpected result: %+v", r)
		}
	})

	t.Run("fenced JSON", func(t *testing.T) {
		var r probeResult
		err := unmarshalRobust("```json\n{\"type\":\"WIKI\",\"language\":\"de\",\"confidence\":0.8}\n```", &r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Type != "WIKI" || r.Language != "de" {
			t.Errorf("unexpected result: %+v", r)
		}
	})

	t.Run("whitespace around JSON", func(t *testing.T) {
		var r probeResult
		err := unmarshalRobust("  \n{\"type\":\"MEMO\",\"language\":\"fr\",\"confidence\":0.7}\n  ", &r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Type != "MEMO" {
			t.Errorf("unexpected type: %q", r.Type)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		var r probeResult
		err := unmarshalRobust("not json at all", &r)
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("fenced invalid JSON returns error", func(t *testing.T) {
		var r probeResult
		err := unmarshalRobust("```json\nnot json\n```", &r)
		if err == nil {
			t.Error("expected error for invalid fenced JSON, got nil")
		}
	})

	t.Run("document result ignores fields it does not ask for", func(t *testing.T) {
		var r documentResult
		raw := `{"fullDocument":"Subject | Detail\n\nBody","headers":["H1","H2"],"language":"en","qualityScore":0.85,"qualityFlags":[]}`
		if err := unmarshalRobust(raw, &r); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.FullDocument != "Subject | Detail\n\nBody" || r.Language != "en" {
			t.Errorf("unexpected result: %+v", r)
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
			name: "real-world LLM output: German email with unescaped close quote",
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
		var r documentResult
		if err := unmarshalRobust(raw2, &r); err != nil {
			t.Fatalf("unmarshalRobust failed on unescaped quote input: %v", err)
		}
		if r.Language != "de" {
			t.Errorf("expected language 'de', got %q", r.Language)
		}
		if r.FullDocument != `Steps "Qlik Sense" done` {
			t.Errorf("fullDocument = %q", r.FullDocument)
		}
		_ = raw
	})

	t.Run("trailing content after JSON object", func(t *testing.T) {
		// json.Decoder handles trailing content; json.Unmarshal does not.
		var r probeResult
		raw := `{"type":"EMAIL","language":"en","confidence":0.95}  extra stuff`
		if err := unmarshalRobust(raw, &r); err != nil {
			t.Fatalf("unmarshalRobust failed on trailing content: %v", err)
		}
		if r.Type != "EMAIL" {
			t.Errorf("expected type EMAIL, got %q", r.Type)
		}
	})
}

// --- buildDocTypePrompt dispatch ---

func TestBuildDocTypePromptDispatch(t *testing.T) {
	for _, dt := range []string{"email", "chat", "wiki", "memo", "powerpoint"} {
		t.Run(dt, func(t *testing.T) {
			sys, user, schema, err := buildDocTypePrompt(dt, 0, "professional", "professional", "", "some text")
			if err != nil {
				t.Fatalf("buildDocTypePrompt: %v", err)
			}
			if sys == "" || len(schema) == 0 {
				t.Error("system prompt and schema must not be empty")
			}
			if user != "some text" {
				t.Errorf("user message should be the input text, got %q", user)
			}
		})
	}
	if _, _, _, err := buildDocTypePrompt("auto", 0, "", "", "", "x"); err == nil {
		t.Error("an unresolved document type reached the prompt builder without an error")
	}
}

// --- prompt variants ---

func TestResolvePromptVariant(t *testing.T) {
	tests := []struct {
		docType string
		variant int
		want    int
		wantErr bool
	}{
		{"email", 0, LatestPromptVariant, false},
		{"memo", 0, LatestPromptVariant, false},
		{"email", 3, 3, false},
		{"chat", 3, 3, false},
		{"email", 2, 2, false},
		{"wiki", 2, 0, true},  // v2 has no wiki
		{"email", 1, 0, true}, // removed with the refine pass
		{"email", 4, 0, true},
	}
	for _, tc := range tests {
		got, err := resolvePromptVariant(tc.docType, tc.variant)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("resolvePromptVariant(%q, %d) = %d, %v; want %d, error %v", tc.docType, tc.variant, got, err, tc.want, tc.wantErr)
		}
	}
	if LatestPromptVariant != 3 {
		t.Errorf("LatestPromptVariant = %d, want 3 — v3 is the default (ADR-003)", LatestPromptVariant)
	}
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

// TestUnmarshalRobustRepairsArrayElements covers the failure that showed up
// reproducibly in the eval: a header whose own text contains quotes —
// `"newformat" jetzt funktional` — lands in the headers array unescaped, and the
// decoder reports `invalid character '(' after array element`. Only strings
// following a ':' used to be repaired, so array elements fell through.
func TestUnmarshalRobustRepairsArrayElements(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			// This is the shape that produced the reported error verbatim: the
			// element opens, a quoted phrase closes the string early, and the
			// decoder meets '(' where it wanted ',' or ']'.
			name: "quoted phrase followed by a parenthetical",
			raw:  `{"fullDocument":"ok","headers":["Views & Reichweite","Views" (Aufrufe)"],"language":"de"}`,
			want: []string{"Views & Reichweite", `Views" (Aufrufe)`},
		},
		{
			name: "quoted phrase mid-element",
			raw:  `{"fullDocument":"ok","headers":[""newformat" jetzt funktional","zweiter Header"],"language":"de"}`,
			want: []string{`"newformat" jetzt funktional`, "zweiter Header"},
		},
		{
			name: "already valid input is untouched",
			raw:  `{"fullDocument":"ok","headers":["a","b"],"language":"de"}`,
			want: []string{"a", "b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var result headedResult
			if err := unmarshalRobust(tc.raw, &result); err != nil {
				t.Fatalf("unmarshalRobust: %v", err)
			}
			if len(result.Headers) != len(tc.want) {
				t.Fatalf("headers = %q, want %q", result.Headers, tc.want)
			}
			for i := range tc.want {
				if result.Headers[i] != tc.want[i] {
					t.Errorf("header %d = %q, want %q", i, result.Headers[i], tc.want[i])
				}
			}
			if result.FullDocument != "ok" || result.Language != "de" {
				t.Errorf("other fields were damaged: %+v", result)
			}
		})
	}
}

// TestUnmarshalRobustKeepsTopLevelArrays guards the leading-content trim: it
// looks for the first '{', which on a top-level array would cut off the opening
// bracket and every element before the first object.
func TestUnmarshalRobustKeepsTopLevelArrays(t *testing.T) {
	var got []map[string]string
	if err := unmarshalRobust(`[{"a":"1"},{"b":"2"}]`, &got); err != nil {
		t.Fatalf("unmarshalRobust: %v", err)
	}
	if len(got) != 2 || got[0]["a"] != "1" || got[1]["b"] != "2" {
		t.Errorf("got %v, want both elements intact", got)
	}
}
