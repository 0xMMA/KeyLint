package pyramidize

import "fmt"

// buildGlobalRefinePrompt builds the system prompt and user message for a
// full-canvas instruction-driven revision. The user instruction takes priority;
// the original text is soft context only and may be overridden.
func buildGlobalRefinePrompt(fullCanvas, originalText, instruction, docType, style, relationship string) (systemPrompt, userMessage string) {
	systemPrompt = fmt.Sprintf(`<role>
You are an expert document editing assistant. You revise documents according to precise user instructions.
</role>

<task>
The user wants to revise the canvas document using the instruction provided. Apply the instruction to produce an improved version of the entire canvas.
</task>

<priorities>
1. The user instruction is the highest priority — follow it exactly and completely
2. The original text is SOFT context only — it may be referenced for background but the user instruction can override or contradict it
3. Preserve the document type (%s), communication style (%s), and relationship level (%s) unless the instruction says otherwise
4. Preserve the language and any code-switching from the existing canvas
</priorities>

<constraints>
- Output the complete revised canvas — not just the changed sections
- Do not add explanations or commentary — output only the document
- Maintain the structural format of the document (subject line / headers / body)
</constraints>

Respond ONLY with valid JSON (no markdown fences, no explanation):
{"newCanvas": "complete revised document text"}`, docType, style, relationship)

	userMessage = fmt.Sprintf("Original text (soft context — may be overridden by instruction):\n%s\n\nCurrent canvas:\n%s\n\nUser instruction:\n%s", originalText, fullCanvas, instruction)
	return
}

// buildSplicePrompt builds the system prompt and user message for rewriting a
// selected section of the canvas. Only the selected section is rewritten; the
// replacement must be coherent with the surrounding canvas.
func buildSplicePrompt(fullCanvas, originalText, selectedText, instruction string) (systemPrompt, userMessage string) {
	systemPrompt = `<role>
You are an expert document editing assistant specialising in precise, in-place text revisions.
</role>

<task>
Rewrite ONLY the selected section of the canvas document according to the user's instruction. The rewritten section must:
1. Follow the instruction exactly
2. Be coherent and flow naturally with the surrounding canvas content
3. Preserve the tone, style, and language of the document
4. Match the structural format expected at that position (e.g. bullet list if replacing bullets, paragraph if replacing prose)
</task>

<constraints>
- Output ONLY the replacement text for the selected section — not the entire canvas
- Do not include surrounding canvas content in your output
- Do not add explanations or commentary
- The original text is provided as background context only — the instruction may override it
</constraints>

Respond ONLY with valid JSON (no markdown fences, no explanation):
{"rewrittenSection": "replacement text for the selected section only"}`

	userMessage = fmt.Sprintf("Original text (background context):\n%s\n\nFull canvas (for coherence reference):\n%s\n\nSelected section to rewrite:\n%s\n\nUser instruction:\n%s", originalText, fullCanvas, selectedText, instruction)
	return
}
