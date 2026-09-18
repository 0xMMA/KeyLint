package pyramidize

import (
	"encoding/json"
	"os"
)

// schemaEnforcement decides whether the schemas below are actually sent. It is
// off by default and read once, at package init.
//
// Enforcing them measurably helps and measurably hurts. Two eval runs on
// Sonnet 4.6 (test-data/eval-runs/2026-09-18T01-50-00 and …T02-00-33, against
// the 0.7680 / 0.8833 baseline from #52):
//
//	deterministic  0.7680 → 0.8025 / 0.8445
//	judge          0.8833 → 0.8223 / 0.8215
//
// The gain is real — email-dataquality-reply-to-feedback, which failed
// reproducibly with a parse error, succeeds. So is the loss, and it is the kind
// a user sees: email-answer-info-update scored 0.1 in both runs, producing 242
// bytes from a 2237-byte input, cut off mid-sentence. That is not an
// output-token limit (truncation is detected separately and raises an error) but
// the model closing the JSON string early under grammar-constrained decoding.
// A half-written document pasted over the user's selection is worse than the
// parse failure the schema fixes, and that failure is fixable in the parser.
//
// The switch stays so E3 (#34) can measure enforcement again on a reworked
// prompt: KEYLINT_PYRAMIDIZE_SCHEMA=1, or ./scripts/eval.sh --schema. See #53
// for why the documented quality baseline is not the one to compare against.
var schemaEnforcement = os.Getenv("KEYLINT_PYRAMIDIZE_SCHEMA") == "1"

// enforcedSchema returns the schema a step should send, or nil while
// enforcement is off.
func enforcedSchema(schema json.RawMessage) json.RawMessage {
	if !schemaEnforcement {
		return nil
	}
	return schema
}

// The shapes every pipeline step asks the model for, as JSON schemas that
// providers can enforce (#47). They mirror the structs in types.go — keep the
// two in step; a field added there and forgotten here simply stops being
// constrained, which is a silent loss rather than a compile error.
//
// Every property is listed in "required" and "additionalProperties" is false,
// because OpenAI's strict mode rejects a schema that leaves either open.
// unmarshalRobust stays the fallback for providers that only approximate the
// schema, or ignore it.

// detectSchema matches detectResult. The lowercase enum is deliberate even
// though prompts_detect.go asks for uppercase: service.go lowercases the answer
// before validating it, so a provider that enforces the schema and one that only
// follows the prompt both end up at the same four values.
var detectSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "type": {"type": "string", "enum": ["email", "wiki", "memo", "powerpoint"]},
    "language": {"type": "string"},
    "confidence": {"type": "number"}
  },
  "required": ["type", "language", "confidence"],
  "additionalProperties": false
}`)

// documentSchema matches foundationResult and refineResult, which are the same
// shape: the refinement step returns a corrected document, not a diff.
var documentSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "fullDocument": {"type": "string"},
    "headers": {"type": "array", "items": {"type": "string"}},
    "language": {"type": "string"},
    "qualityScore": {"type": "number"},
    "qualityFlags": {"type": "array", "items": {"type": "string"}}
  },
  "required": ["fullDocument", "headers", "language", "qualityScore", "qualityFlags"],
  "additionalProperties": false
}`)

// documentSchemaV2 matches what the v2 email prompt actually produces. That
// prompt drops the self-QA block on purpose — "quality is evaluated externally
// by deterministic checks and LLM-as-judge" — so forcing qualityScore and
// qualityFlags would make the model invent both, and an invented low score with
// any invented flag switches on the refinement round-trip the variant exists to
// avoid. That would quietly change what the v1-vs-v2 eval is comparing.
var documentSchemaV2 = json.RawMessage(`{
  "type": "object",
  "properties": {
    "fullDocument": {"type": "string"},
    "headers": {"type": "array", "items": {"type": "string"}},
    "language": {"type": "string"}
  },
  "required": ["fullDocument", "headers", "language"],
  "additionalProperties": false
}`)

// canvasSchema matches canvasResult.
var canvasSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "newCanvas": {"type": "string"}
  },
  "required": ["newCanvas"],
  "additionalProperties": false
}`)

// spliceSchema matches spliceResult.
var spliceSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "rewrittenSection": {"type": "string"}
  },
  "required": ["rewrittenSection"],
  "additionalProperties": false
}`)

// judgeSchema matches JudgeScore in eval_judge.go — the eval's LLM-as-judge
// call goes through the same provider layer and benefits from the same
// constraint.
var judgeSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "pyramidStructure": {"type": "number"},
    "clarity": {"type": "number"},
    "completeness": {"type": "number"},
    "tonePreservation": {"type": "number"},
    "overall": {"type": "number"},
    "rationale": {"type": "string"}
  },
  "required": ["pyramidStructure", "clarity", "completeness", "tonePreservation", "overall", "rationale"],
  "additionalProperties": false
}`)
