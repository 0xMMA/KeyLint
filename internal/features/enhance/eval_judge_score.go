package enhance

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Kept out of the eval-tagged file on purpose: the API call needs a key and a
// build tag, but turning a response into a score is ordinary parsing and has to
// be covered by the normal test suite.

// JudgeScore is one sample's judged result.
type JudgeScore struct {
	Correctness      float64 `json:"correctness"`
	MeaningPreserved float64 `json:"meaningPreserved"`
	TonePreserved    float64 `json:"tonePreserved"`
	NoOverEditing    float64 `json:"noOverEditing"`
	Overall          float64 `json:"overall"`
	Rationale        string  `json:"rationale"`
}

// rawJudgeScore takes the five numbers as pointers so that "the judge did not
// answer this dimension" and "the judge scored it zero" stay different facts.
//
// They were the same fact until now: a response missing `overall` unmarshalled
// to 0.0 and was averaged in as the worst possible score, which moves the
// headline number for a reason that has nothing to do with the candidate. An
// error instead leaves judgeCount below sampleCount, and the aggregate already
// refuses to compare a run whose judge skipped samples.
type rawJudgeScore struct {
	Correctness      *float64 `json:"correctness"`
	MeaningPreserved *float64 `json:"meaningPreserved"`
	TonePreserved    *float64 `json:"tonePreserved"`
	NoOverEditing    *float64 `json:"noOverEditing"`
	Overall          *float64 `json:"overall"`
	Rationale        string   `json:"rationale"`
}

func parseJudgeScore(text string) (JudgeScore, error) {
	var raw rawJudgeScore
	if err := json.Unmarshal([]byte(stripCodeFence(text)), &raw); err != nil {
		return JudgeScore{}, fmt.Errorf("judge parse error: %w", err)
	}
	fields := []struct {
		name string
		val  *float64
	}{
		{"correctness", raw.Correctness},
		{"meaningPreserved", raw.MeaningPreserved},
		{"tonePreserved", raw.TonePreserved},
		{"noOverEditing", raw.NoOverEditing},
		{"overall", raw.Overall},
	}
	for _, f := range fields {
		if f.val == nil {
			return JudgeScore{}, fmt.Errorf("judge response has no %s", f.name)
		}
		if *f.val < 0 || *f.val > 1 {
			return JudgeScore{}, fmt.Errorf("judge scored %s at %v, outside 0–1", f.name, *f.val)
		}
	}
	return JudgeScore{
		Correctness:      *raw.Correctness,
		MeaningPreserved: *raw.MeaningPreserved,
		TonePreserved:    *raw.TonePreserved,
		NoOverEditing:    *raw.NoOverEditing,
		Overall:          *raw.Overall,
		Rationale:        raw.Rationale,
	}, nil
}

// fixJudgeSchema constrains the reply to the shape JudgeScore parses, the way
// the Pyramidize judge does. The request used to set JSONMode, which asks for
// "some JSON" and is a no-op on Anthropic — the fenced replies that cost the
// first baseline its judge scores were the visible half of that.
var fixJudgeSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "correctness": {"type": "number"},
    "meaningPreserved": {"type": "number"},
    "tonePreserved": {"type": "number"},
    "noOverEditing": {"type": "number"},
    "overall": {"type": "number"},
    "rationale": {"type": "string"}
  },
  "required": ["correctness", "meaningPreserved", "tonePreserved", "noOverEditing", "overall", "rationale"],
  "additionalProperties": false
}`)

// stripCodeFence pulls the JSON object out of whatever the model wrapped it in.
//
// The schema makes this unnecessary on providers that enforce one, and it is
// kept because not every provider does: the first baseline run lost every judge
// score to a ```json fence. Two shapes it used to miss — a fence opened and
// closed on one line, and a sentence of prose before the fence — are why it
// ends by taking the outermost braces rather than trusting what it trimmed.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "```"); i >= 0 {
		rest := s[i+3:]
		if j := strings.LastIndex(rest, "```"); j >= 0 {
			rest = rest[:j]
		}
		rest = strings.TrimLeft(rest, " \t")
		if k := strings.IndexByte(rest, '\n'); k >= 0 && isLangTag(strings.TrimSpace(rest[:k])) {
			rest = rest[k+1:]
		} else if sp := strings.IndexAny(rest, " \t"); sp > 0 && isLangTag(rest[:sp]) {
			rest = rest[sp+1:]
		}
		s = strings.TrimSpace(rest)
	}
	// Prose on either side of the object, fenced or not.
	if a, b := strings.Index(s, "{"), strings.LastIndex(s, "}"); a >= 0 && b > a {
		s = s[a : b+1]
	}
	return s
}

// isLangTag reports whether a fence opener looks like "json" rather than the
// first line of the object.
func isLangTag(s string) bool {
	if s == "" || len(s) > 12 {
		return false
	}
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}
