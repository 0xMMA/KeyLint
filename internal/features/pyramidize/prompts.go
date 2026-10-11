package pyramidize

import (
	"encoding/json"
	"fmt"
)

// LatestPromptVariant is what a request's PromptVariant 0 means: v3, one
// shared core plus a section per form (prompts_v3.go, ADR-003).
//
// Variant 2 is the v2 email prompt, kept for the v3-vs-v2 measurement and for
// email only — it has no other form. Variant 1 (email with self-QA, and the
// refine pass it fed) and the memo/wiki/powerpoint pipeline prompts were
// removed with that pass: without it none of them was the pipeline ADR-002
// measured any more.
const LatestPromptVariant = 3

// resolvePromptVariant maps a request's variant onto the prompt that runs for
// docType, or says why none does. 0 (or less) means the latest.
func resolvePromptVariant(docType string, variant int) (int, error) {
	if variant <= 0 {
		return LatestPromptVariant, nil
	}
	switch variant {
	case 3:
		return 3, nil
	case 2:
		if docType != "email" {
			return 0, fmt.Errorf("prompt variant 2 exists for email only, not %q", docType)
		}
		return 2, nil
	case 1:
		return 0, fmt.Errorf("prompt variant 1 was removed with the refine pass (ADR-003); 2 is the previous email prompt")
	default:
		return 0, fmt.Errorf("unknown prompt variant %d (0 = latest = %d, or 2 for the v2 email prompt)", variant, LatestPromptVariant)
	}
}

// buildDocTypePrompt returns the prompt and the reply schema for one run.
// docType must already be resolved to a form (see resolveDocType).
func buildDocTypePrompt(docType string, variant int, style, relationship, customInstructions, text string) (systemPrompt, userMessage string, schema json.RawMessage, err error) {
	v, err := resolvePromptVariant(docType, variant)
	if err != nil {
		return "", "", nil, err
	}
	if v == 2 {
		system, user := buildEmailPromptV2(style, relationship, customInstructions, text)
		return system, user, documentSchemaV2, nil
	}
	system, user, err := buildPromptV3(docType, style, relationship, customInstructions, text)
	if err != nil {
		return "", "", nil, err
	}
	return system, user, documentSchemaV3, nil
}
