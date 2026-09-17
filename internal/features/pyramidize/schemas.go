package pyramidize

import "encoding/json"

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
