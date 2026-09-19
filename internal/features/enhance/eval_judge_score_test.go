package enhance

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const judgeObj = `{"correctness":0.9,"meaningPreserved":1,"tonePreserved":1,"noOverEditing":0.8,"overall":0.9,"rationale":"ok"}`

// TestJudgeRepliesAreUnwrapped: the judge is told to answer with JSON only and
// wraps it anyway. The first baseline run lost every judge score to a ```json
// fence; a later review found two more shapes the unwrapper missed.
func TestJudgeRepliesAreUnwrapped(t *testing.T) {
	for name, raw := range map[string]string{
		"plain":              judgeObj,
		"json fence":         "```json\n" + judgeObj + "\n```",
		"bare fence":         "```\n" + judgeObj + "\n```",
		"fence with space":   "  ```json\n" + judgeObj + "\n```  ",
		"single-line fence":  "```json " + judgeObj + "```",
		"single-line bare":   "```" + judgeObj + "```",
		"prose before fence": "Here is my assessment:\n\n```json\n" + judgeObj + "\n```",
		"prose, no fence":    "Sure, here it is: " + judgeObj,
		"trailing prose":     "```json\n" + judgeObj + "\n```\n\nLet me know if you need more.",
	} {
		got := stripCodeFence(raw)
		if got != judgeObj {
			t.Errorf("%s: got %q", name, got)
		}
		if _, err := parseJudgeScore(raw); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestAMissingJudgeDimensionIsAnError: every number was a plain float64, so a
// reply that left one out unmarshalled to 0.0 and was averaged in as the worst
// possible score — a measurement of the judge's formatting, reported as the
// candidate's quality.
func TestAMissingJudgeDimensionIsAnError(t *testing.T) {
	for _, field := range []string{"overall", "correctness", "noOverEditing"} {
		var obj map[string]any
		if err := json.Unmarshal([]byte(judgeObj), &obj); err != nil {
			t.Fatal(err)
		}
		delete(obj, field)
		raw, _ := json.Marshal(obj)

		if _, err := parseJudgeScore(string(raw)); err == nil {
			t.Errorf("a reply without %q was accepted", field)
		} else if !strings.Contains(err.Error(), field) {
			t.Errorf("the error for a missing %q does not name it: %v", field, err)
		}
	}

	// A genuine zero is a score, not an absence.
	score, err := parseJudgeScore(`{"correctness":0,"meaningPreserved":0,"tonePreserved":0,"noOverEditing":0,"overall":0,"rationale":"wrong"}`)
	if err != nil {
		t.Fatalf("a reply scoring zero was rejected: %v", err)
	}
	if score.Overall != 0 {
		t.Errorf("overall = %v, want 0", score.Overall)
	}

	// And a number outside the scale is not one either.
	if _, err := parseJudgeScore(`{"correctness":1,"meaningPreserved":1,"tonePreserved":1,"noOverEditing":1,"overall":9.5,"rationale":"x"}`); err == nil {
		t.Error("a score of 9.5 on a 0-1 scale was accepted")
	}
}

// TestTheJudgeSchemaMirrorsJudgeScore: the schema is what the provider enforces,
// so a field that drifts from the struct is a field the judge is told to send
// and nothing reads. Mirrors the Pyramidize suite's check on its own schemas.
func TestTheJudgeSchemaMirrorsJudgeScore(t *testing.T) {
	var parsed struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
		Additional *bool                      `json:"additionalProperties"`
	}
	if err := json.Unmarshal(fixJudgeSchema, &parsed); err != nil {
		t.Fatalf("the schema is not valid JSON: %v", err)
	}

	var properties []string
	for name := range parsed.Properties {
		properties = append(properties, name)
	}
	var fields []string
	rt := reflect.TypeOf(JudgeScore{})
	for i := 0; i < rt.NumField(); i++ {
		fields = append(fields, strings.Split(rt.Field(i).Tag.Get("json"), ",")[0])
	}
	sort.Strings(properties)
	sort.Strings(fields)
	required := append([]string(nil), parsed.Required...)
	sort.Strings(required)

	if !reflect.DeepEqual(properties, fields) {
		t.Errorf("schema properties = %v, want the struct's fields %v", properties, fields)
	}
	if !reflect.DeepEqual(required, properties) {
		t.Errorf("required = %v, want every property %v — an optional one is what parseJudgeScore refuses", required, properties)
	}
	if parsed.Additional == nil || *parsed.Additional {
		t.Error("additionalProperties must be false")
	}
}
