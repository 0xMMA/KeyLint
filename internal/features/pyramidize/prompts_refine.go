package pyramidize

import (
	"fmt"
	"strings"
)

// buildRefinePrompt builds the system prompt and user message for the optional
// refinement call. It is only invoked when the foundation output scores below the
// quality threshold. Each flag in flags generates targeted remediation instructions.
func buildRefinePrompt(originalText, failedOutput string, flags []string) (systemPrompt, userMessage string) {
	var sb strings.Builder

	sb.WriteString(`<role>
You are an expert document editor performing a targeted quality pass on a previously generated document.
</role>

<task>
The document below was generated from the original text but scored below the quality threshold. Your job is to fix the specific quality issues identified by the quality flags while preserving everything that is already good.
</task>

<original_text_constraint>
HARD CONSTRAINT: The original text is the source of truth. Every person, deadline, action, decision, and piece of information present in the original text MUST appear in your output. You may never omit content from the original text.
</original_text_constraint>

<quality_issues_to_fix>
`)

	for _, flag := range flags {
		switch flag {
		case "MECE_VIOLATION":
			sb.WriteString("- MECE_VIOLATION: The headers are not MECE. Restructure the headers so each one is mutually exclusive (no overlap in content) and collectively exhaustive (no major topic left uncovered). Every content point must belong to exactly one header section.\n")
		case "INFO_LOSS":
			sb.WriteString("- INFO_LOSS: Content from the original is missing in the generated document. Cross-check every person, deadline, action, decision, and piece of information from the original text and ensure it is present in the output. The original text is a hard constraint — nothing may be omitted.\n")
		case "STYLE_MISMATCH":
			sb.WriteString("- STYLE_MISMATCH: The tone or language style does not match the requested communication style. Review every sentence and ensure the register, formality, and vocabulary are consistent with the requested style and relationship level throughout the document.\n")
		case "SUBJECT_FORMAT":
			sb.WriteString("- SUBJECT_FORMAT: The subject line or title (first line of fullDocument) does not follow the required format. Fix it: it must be information-dense and follow the prescribed format for this document type (pipe-separated fields for email, # H1 for wiki, **TO/FROM/RE** line for memo, # Slide N: for powerpoint).\n")
		case "FIDELITY_VIOLATION":
			sb.WriteString("- FIDELITY_VIOLATION: The document adds claims, conclusions, or editorial statements not present in the original. Remove any content not directly supported by the original text. Cross-check every assertion against the source — if it is not there, delete it.\n")
		default:
			sb.WriteString(fmt.Sprintf("- %s: Address this quality issue in the output.\n", flag))
		}
	}

	sb.WriteString(`</quality_issues_to_fix>

<instructions>
- Fix only the identified issues — do not restructure content that is already correct
- Preserve the document type and overall structure from the previous attempt
- The original text is a HARD constraint for completeness — add back anything that was lost
- Language: preserve the exact language (and any code-switching) from the original text
- Produce the complete revised document, not just the changed sections
</instructions>

`)

	sb.WriteString("Respond ONLY with valid JSON (no markdown fences, no explanation):\n")
	sb.WriteString(`{
  "fullDocument": "complete revised document text with subject/title as first line",
  "headers": ["header 1", "header 2"],
  "language": "en",
  "qualityScore": 0.0,
  "qualityFlags": []
}`)

	systemPrompt = sb.String()

	userMessage = fmt.Sprintf("Original text (hard constraint — preserve all content):\n%s\n\nPrevious output (to be improved):\n%s", originalText, failedOutput)
	return
}
