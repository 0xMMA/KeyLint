package pyramidize

// selfQABlock is appended to all document-type foundation prompts.
// It instructs the model to evaluate its own output through five specialist lenses
// and compute a qualityScore + qualityFlags before responding.
const selfQABlock = `
<self_evaluation>
After generating the document, evaluate it as five internal specialists:

1. Subject/Title Specialist: Does the first line follow the required format exactly? Is it information-dense? Score: 0–1
2. MECE Specialist: Are headers at the same level mutually exclusive (no overlap) and collectively exhaustive (no major gap)? Score: 0–1
3. Completeness Specialist: Is every person, deadline, action, and decision from the original preserved? Risk: 0–1 (0=nothing lost)
4. Style Specialist: Is the language consistent with the requested style and relationship level? Is code-switching preserved? Score: 0–1
5. Fidelity Specialist: Does the output add any claims, conclusions, or editorial statements NOT present in the original text? Risk: 0–1 (0=perfectly faithful, 1=significant additions)

qualityScore = (subject_score + mece_score + style_score) / 3 - (completeness_risk * 0.2) - (fidelity_risk * 0.2)

qualityFlags: include any applicable: "MECE_VIOLATION", "INFO_LOSS", "STYLE_MISMATCH", "SUBJECT_FORMAT", "FIDELITY_VIOLATION"
</self_evaluation>

Respond ONLY with valid JSON (no markdown fences, no explanation, no trailing text).
CRITICAL: All double-quote characters inside string values MUST be escaped as \" — this includes quotes in the document text, headers, and any other string fields.
{
  "fullDocument": "complete document text with subject/title as first line",
  "headers": ["header 1", "header 2"],
  "language": "en",
  "qualityScore": 0.0,
  "qualityFlags": []
}`
