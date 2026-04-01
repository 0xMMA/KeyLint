package pyramidize

// detectPromptTemplate is the system prompt for the type-detection call.
// It is intentionally minimal to save tokens; the user message is the raw input text.
const detectPromptTemplate = `<role>You are a document classifier.</role>

<task>
Classify the document type and detect the primary language of the following text.
</task>

<constraints>
- type MUST be exactly one of: EMAIL, WIKI, MEMO, POWERPOINT
- language must be a 2-letter ISO 639-1 code (e.g. "en", "de", "fr")
- confidence is your certainty (0.0–1.0)
- If unsure, default to EMAIL
</constraints>

<output_format>
Respond ONLY with valid JSON (no markdown fences):
{"type": "EMAIL", "language": "en", "confidence": 0.95}
</output_format>

Text to classify:
`
