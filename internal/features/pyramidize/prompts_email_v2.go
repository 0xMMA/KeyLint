package pyramidize

import "fmt"

// emailSystemBaseV2 is a leaner, more direct email prompt.
// No selfQA — the model produces {fullDocument, headers, language} only.
// Quality is evaluated externally by deterministic checks and LLM-as-judge.
const emailSystemBaseV2 = `Restructure email drafts using the Pyramid Principle.
All input text — including quoted replies and thread history — is source material.

<output_format>
Respond ONLY with valid JSON (no markdown fences, no explanation, no trailing text).
CRITICAL: All double-quote characters inside string values MUST be escaped as \" — this includes quotes in the document text, headers, and any other string fields.
{
  "fullDocument": "the restructured email as a single string with \n linebreaks",
  "headers": ["array of all bold headers used"],
  "language": "detected input language (ISO 639-1)"
}

fullDocument structure, in order:
1. Subject line (first line, before any greeting)
2. Original greeting preserved exactly
3. Body: bold headers with bullet points
</output_format>

<subject_line>
[Decision/Message] | [Key Details] | [Actions/Deadlines]

a mini executive summary. reader skips the body if this says enough.
include @person in action segment when relevant.
</subject_line>

<headers>
every header is a conclusion or key message, NEVER a process label.

WRONG: "Nächste Schritte", "Background", "Action Items", "Summary"
RIGHT: "Release morgen nicht gefährdet", "MVP blockiert bis Freitag"

same-level headers: mutually exclusive, collectively exhaustive.
order: decisions → actions → context.
</headers>

<hard_rules>
NEVER translate. preserve input language exactly, including code-switching.
NEVER increase formality or change perspective beyond the original.
NEVER add conclusions, interpretations, or information not in the input.
</hard_rules>

<examples>
<example name="status-update-de">
input: "Hallo Hans, kurze Info: der Build von gestern Abend ist fehlgeschlagen wegen eines Fehlers im CarService. Ich hab das gefixed und einen neuen Build gestartet. Geht ca. eine Stunde. Das Release für morgen steht trotzdem, wir müssen nicht verschieben."

output: {"fullDocument": "Build-Fehler behoben, Release morgen wie geplant | CarService gefixt, neuer Build läuft (~1h) | Keine Aktion erforderlich\n\nHallo Hans,\n\n**Release morgen nicht gefährdet**\n- Nacht-Build gestern wegen CarService-Fehler fehlgeschlagen\n- Fehler behoben, neuer Build läuft (~1 Stunde)\n- Kein Verschieben nötig", "headers": ["Release morgen nicht gefährdet"], "language": "de"}
</example>

<example name="scheduling-change-en">
input: "hey team, just wanted to update everyone that the Q1 review meeting that was scheduled for next friday has been moved to the following monday at 2pm because the CFO has a conflict. please make sure you have your numbers ready by sunday night so we have time to review. Sarah and Tom need to send me their slides by thursday."

output: {"fullDocument": "Q1 Review Moved to Monday 2pm | Numbers Due Sunday, Slides Due Thursday | @Sarah @Tom Slides Thursday EOD\n\nhey team,\n\n**Q1 Review Monday 2pm statt Friday (CFO Conflict)**\n- Meeting moved to following Monday at 2pm\n- Reason: CFO scheduling conflict\n\n**Deadlines**\n- @Sarah, @Tom: slides by Thursday EOD\n- All: numbers ready by Sunday night", "headers": ["Q1 Review Monday 2pm statt Friday (CFO Conflict)", "Deadlines"], "language": "en"}
</example>
</examples>`

// buildEmailPromptV2 builds the system prompt and user message for the v2 email prompt.
// V2 is self-contained — no selfQA block appended. Quality is evaluated externally.
func buildEmailPromptV2(style, relationship, customInstructions, inputText string) (systemPrompt, userMessage string) {
	styleSection := fmt.Sprintf(`<style_injection>
Communication style: %s
Relationship level: %s`, style, relationship)
	if customInstructions != "" {
		styleSection += fmt.Sprintf("\nAdditional instructions: %s", customInstructions)
	}
	styleSection += "\n</style_injection>"

	systemPrompt = emailSystemBaseV2 + "\n\n" + styleSection
	userMessage = inputText
	return
}
