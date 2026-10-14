package pyramidize

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// The schemas are hand-written and the header comment in schemas.go admits that
// a field added to types.go and forgotten here simply stops being constrained.
// These tests make that a failure instead: they compare every schema against the
// struct it mirrors, by reflection.

// structJSONFields lists the json tag names of a struct's exported fields.
func structJSONFields(t *testing.T, v any) []string {
	t.Helper()
	typ := reflect.TypeOf(v)
	var names []string
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			t.Fatalf("%s.%s has no json tag; the schema cannot mirror it", typ.Name(), typ.Field(i).Name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// schemaFields returns a schema's property names and its required list.
func schemaFields(t *testing.T, raw json.RawMessage) (properties, required []string) {
	t.Helper()
	var parsed struct {
		Type                 string                     `json:"type"`
		Properties           map[string]json.RawMessage `json:"properties"`
		Required             []string                   `json:"required"`
		AdditionalProperties *bool                      `json:"additionalProperties"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	if parsed.Type != "object" {
		t.Errorf("schema type = %q, want object", parsed.Type)
	}
	// OpenAI's strict mode rejects a schema that leaves either of these open.
	if parsed.AdditionalProperties == nil || *parsed.AdditionalProperties {
		t.Error("schema must set additionalProperties:false for OpenAI strict mode")
	}
	for name := range parsed.Properties {
		properties = append(properties, name)
	}
	required = append(required, parsed.Required...)
	sort.Strings(properties)
	sort.Strings(required)
	if !reflect.DeepEqual(properties, required) {
		t.Errorf("required = %v, want every property %v — strict mode rejects an optional one", required, properties)
	}
	return properties, required
}

func TestSchemasMirrorTheStructsTheyParseInto(t *testing.T) {
	tests := []struct {
		name   string
		schema json.RawMessage
		target any
	}{
		{"document v3", documentSchemaV3, documentResult{}},
		{"canvas", canvasSchema, canvasResult{}},
		{"splice", spliceSchema, spliceResult{}},
		{"judge", judgeSchema, JudgeScore{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			properties, _ := schemaFields(t, tc.schema)
			want := structJSONFields(t, tc.target)
			if !reflect.DeepEqual(properties, want) {
				t.Errorf("schema properties = %v, want the struct's fields %v", properties, want)
			}
		})
	}
}

// TestDocumentSchemaV2MatchesTheV2Prompt: v2 is kept as it was measured, so
// its schema keeps the three fields its prompt asks for — and documentResult
// must be able to hold the two that are read.
func TestDocumentSchemaV2MatchesTheV2Prompt(t *testing.T) {
	properties, _ := schemaFields(t, documentSchemaV2)
	want := []string{"fullDocument", "headers", "language"}
	if !reflect.DeepEqual(properties, want) {
		t.Errorf("documentSchemaV2 properties = %v, want %v", properties, want)
	}
	for _, name := range structJSONFields(t, documentResult{}) {
		found := false
		for _, p := range properties {
			found = found || p == name
		}
		if !found {
			t.Errorf("v2 replies carry no %q, which documentResult reads", name)
		}
	}
	for _, dropped := range []string{"qualityScore", "qualityFlags"} {
		if strings.Contains(string(documentSchemaV2), dropped) || strings.Contains(string(documentSchemaV3), dropped) {
			t.Errorf("a document schema asks for %q, which no prompt defines", dropped)
		}
	}
}

// TestSchemasSurviveTheCommandLine guards the Claude Code path: the schema
// becomes an argv element, and cmd.exe ends a command at the first newline.
func TestSchemasSurviveTheCommandLine(t *testing.T) {
	all := map[string]json.RawMessage{
		"documentV3": documentSchemaV3,
		"documentV2": documentSchemaV2,
		"canvas":     canvasSchema,
		"splice":     spliceSchema,
		"judge":      judgeSchema,
	}
	for name, schema := range all {
		t.Run(name, func(t *testing.T) {
			var compact bytes.Buffer
			if err := json.Compact(&compact, schema); err != nil {
				t.Fatalf("schema does not compact: %v", err)
			}
			if strings.ContainsAny(compact.String(), "\n\r") {
				t.Errorf("compacted schema still carries a line break: %q", compact.String())
			}
			// cmd.exe's command-line limit; the client sends this on argv.
			if compact.Len() > 8000 {
				t.Errorf("compacted schema is %d bytes, too close to the Windows command-line limit", compact.Len())
			}
		})
	}
}

// TestEnforcedSchemaRespectsTheSwitch pins the gate itself: off means no schema
// travels, on means the caller's schema does.
func TestEnforcedSchemaRespectsTheSwitch(t *testing.T) {
	original := schemaEnforcement
	t.Cleanup(func() { schemaEnforcement = original })

	schemaEnforcement = false
	if got := enforcedSchema(documentSchemaV3); got != nil {
		t.Errorf("enforcedSchema = %s, want nil while enforcement is off", got)
	}

	schemaEnforcement = true
	if got := enforcedSchema(documentSchemaV3); !bytes.Equal(got, documentSchemaV3) {
		t.Errorf("enforcedSchema = %s, want the caller's schema while enforcement is on", got)
	}
}
