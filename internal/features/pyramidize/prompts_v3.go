package pyramidize

import "fmt"

// Prompt v3 (ADR-003): one shared core plus one section per form. The user
// picks the form; one call writes it. There is no self-QA block and no
// quality field to ask for, so nothing in the reply can buy a second call.
//
// The core is v2-email's rules (the measured, shipped prompt), generalised to
// every form and restructured: the pyramid, content-statement headers,
// fidelity, the author's voice and language. Each form section says only what
// differs: the shape of the document and its first line.
//
// General principles only. Nothing here is tuned to an eval sample — the
// examples are the prompt's own, and each one is checked against its input:
// every fact in the output is in the input, and nothing else is.

// promptV3Core is shared by every form.
const promptV3Core = `Restructure the user's text using the Pyramid Principle, in the form described in <form>.
All input text — including quoted replies, forwarded messages and thread history — is source material.

<pyramid>
lead with the main point: the decision, conclusion, request or status the reader needs most.
then the support, grouped: each group backs up the main point, groups do not overlap, together they cover the whole input.
order the groups by what the reader needs: decisions → actions → context.
earlier messages in a thread are context: they move to the end, they do not drop out.
</pyramid>

<headers>
every header is a conclusion or key message — understandable without the text below it. NEVER a process label.

WRONG: "Nächste Schritte", "Background", "Action Items", "Summary"
RIGHT: "Release morgen nicht gefährdet", "MVP blockiert bis Freitag"

same-level headers: mutually exclusive, collectively exhaustive.
</headers>

<fidelity>
every fact, name, number, date, deadline, request and open question in the input appears in the output.
NEVER add conclusions, interpretations, reasons, recommendations or information not in the input.
NEVER add status claims the input does not make, such as "no action needed" or "everything on track".
what the input leaves open stays open: do not resolve it by guessing.
</fidelity>

<voice>
NEVER translate. write in the language of the input, including its code-switching: where the author mixes languages, mix them the same way; where the author does not, do not introduce another language.
NEVER increase formality or change perspective beyond the original: casual stays casual, first person stays first person.
keep greetings and sign-offs as the author wrote them, where the form has them. never upgrade or invent one.
</voice>

<style>
compact but complete: no filler, no courtesy phrases the input does not have.
as much structure as the content needs: a short message gets a short result, never padding to fill a template.
the communication style and relationship in <style_injection> tune word choice, directness and how much context to spell out for that reader. they never change the content and never raise the formality above the author's.
additional instructions in <style_injection> come from the user: follow them over the form's defaults, never over fidelity.
</style>`

// promptV3Output is the reply shape every form shares: the document and its
// language. Headers, subject and title live inside the document, where the
// user sees them; nothing reads a separate list of them.
const promptV3Output = `<output_format>
Respond ONLY with valid JSON (no markdown fences, no explanation, no trailing text).
CRITICAL: All double-quote characters inside string values MUST be escaped as \" — this includes quotes in the document text.
{
  "fullDocument": "the restructured text as a single string with \n linebreaks",
  "language": "language of the input (ISO 639-1)"
}
</output_format>`

// promptV3Forms holds one section per form, keyed by document type. The
// examples are written to the core's rules: in the email one, the first
// subject has no actions segment because its input asks nothing of anyone.
var promptV3Forms = map[string]string{
	"email": `<form name="email">
an email. fullDocument, in order:
1. subject line (first line, before any greeting)
2. the original greeting, exactly as written — none if the input has none
3. body: bold headers (**…**) on their own line, each followed by bullet points
4. the original sign-off, exactly as written — none if the input has none

subject line:
[Decision/Message] | [Key Details] | [Actions/Deadlines]
a mini executive summary: the reader skips the body if this says enough.
include @person in the actions segment when the input asks something of someone.
leave the actions segment out when the input asks nothing of anyone — never fill it with "no action needed".

<examples>
<example name="status-update-de">
input: "Hallo Hans, kurze Info: der Build von gestern Abend ist fehlgeschlagen wegen eines Fehlers im CarService. Ich hab das gefixed und einen neuen Build gestartet. Geht ca. eine Stunde. Das Release für morgen steht trotzdem, wir müssen nicht verschieben."

output: {"fullDocument": "Build-Fehler behoben, Release morgen wie geplant | CarService gefixt, neuer Build läuft (~1h)\n\nHallo Hans,\n\n**Release morgen nicht gefährdet**\n- Build gestern Abend wegen CarService-Fehler fehlgeschlagen\n- Fehler behoben, neuer Build läuft (~1 Stunde)\n- Kein Verschieben nötig", "language": "de"}
</example>

<example name="scheduling-change-en">
input: "hey team, just wanted to update everyone that the Q1 review meeting that was scheduled for next friday has been moved to the following monday at 2pm because the CFO has a conflict. please make sure you have your numbers ready by sunday night so we have time to review. Sarah and Tom need to send me their slides by thursday."

output: {"fullDocument": "Q1 Review Moved to Monday 2pm | Numbers Due Sunday Night | @Sarah @Tom Slides by Thursday\n\nhey team,\n\n**Q1 Review Moved to the Following Monday, 2pm (CFO Conflict)**\n- Was scheduled for next Friday\n- Moved because the CFO has a conflict\n\n**Numbers Due Sunday Night, Slides Due Thursday**\n- All: numbers ready by Sunday night, so there is time to review\n- @Sarah, @Tom: send me your slides by Thursday", "language": "en"}
</example>
</examples>
</form>`,

	"chat": `<form name="chat">
a chat message (Teams, Slack and the like): short, read in a few seconds.
- no subject line, no headers, no bold section titles
- the first sentence carries the main point; an ask goes in it or right after it
- separate points may follow as a short list; one point stays one sentence
- greeting and sign-off only if the input has them, as written — never add a formula
- shorten the wording freely, but every fact and every ask stays

<example name="deployment-de">
input: "Hi Tom, kurzes Update zum Deployment: hat gestern Abend nicht geklappt, weil das Zertifikat abgelaufen war. Hab ein neues beantragt, sollte bis morgen Mittag da sein. Danach deploye ich direkt. Kannst du bis dahin den Kunden informieren? Danke!"

output: {"fullDocument": "Hi Tom, das Deployment kommt erst mit dem neuen Zertifikat (sollte bis morgen Mittag da sein) – kannst du bis dahin den Kunden informieren?\n\nGestern Abend ist es gescheitert, weil das Zertifikat abgelaufen war. Das neue ist beantragt, danach deploye ich direkt.\n\nDanke!", "language": "de"}
</example>
</form>`,

	"memo": `<form name="memo">
an internal memo. fullDocument, in order:
1. header line (first line, all on one line): **TO:** [recipient] | **FROM:** [sender] | **RE:** [the key message as a statement]
   labels in the language of the input (German: **AN:** | **VON:** | **BETREFF:**).
   recipient or sender the input does not name: keep the brackets as a placeholder — never invent a name.
   RE states the message, not the topic. WRONG: "RE: Project Update" RIGHT: "RE: Q2 Launch Slips Two Weeks — Scope Decision Needed by Friday"
2. one sentence that states the key point
3. body: bold headers (**…**) on their own line, each followed by bullet points
no greeting, no sign-off: a memo has neither.

<example name="api-switch-en">
input: "Need to tell the team that we're switching from REST to GraphQL for the new client portal. The decision was made because of performance issues with the current approach - we're making too many round trips. Timeline is 6 weeks starting next monday. Jake and Priya will lead the migration. Everyone else needs to attend a kickoff meeting on friday."

output: {"fullDocument": "**TO:** Team | **FROM:** [sender] | **RE:** Client Portal Moves From REST to GraphQL — Kickoff Friday, Start Monday\n\nThe new client portal switches from REST to GraphQL over six weeks from next Monday, because the current approach makes too many round trips.\n\n**Everyone Else Attends the Kickoff on Friday**\n- Everyone not leading the migration attends the kickoff meeting on Friday\n\n**Jake and Priya Lead the Migration, Six Weeks From Next Monday**\n- Start: next Monday, duration: 6 weeks\n- Migration lead: Jake and Priya\n\n**Too Many Round Trips Drive the Switch to GraphQL**\n- Performance issues with the current REST approach\n- Cause: too many round trips", "language": "en"}
</example>
</form>`,

	"wiki": `<form name="wiki">
a wiki page: reference text for any reader, read now or in a year. fullDocument, in order:
1. title (first line): # [precise, specific title that states the topic and its key fact]
2. one sentence that states the key insight
3. sections: ## headers, each followed by bullet points; the most important section first
- code, commands and configuration go in code blocks
- no greeting, no sign-off, no personal address: they belong to a message, not a page. keep every fact they carried.
- keep source references and links from the input

<example name="logout-bug-en">
input: "so we had this bug where users were getting logged out randomly. turned out it was because the session token wasnt being refreshed properly. the token expires after 30 mins and we refresh it on every api call but there was a race condition in the refresh logic. fixed it with a mutex. this affects all users on v2.3.1 and below."

output: {"fullDocument": "# Race Condition in Session Token Refresh Logged Users Out at Random (v2.3.1 and Below)\n\nRandom logouts came from a race condition in the session token refresh, fixed with a mutex.\n\n## A Mutex Fixes the Race Condition in the Refresh Logic\n- The session token was not refreshed properly\n- Cause: a race condition in the refresh logic\n- Fix: a mutex\n\n## All Users on v2.3.1 and Below Are Affected\n- Symptom: users logged out at random\n\n## The Token Expires After 30 Minutes and Is Refreshed on Every API Call\n- Expiry: 30 minutes\n- Refresh: on every API call", "language": "en"}
</example>
</form>`,

	"powerpoint": `<form name="presentation">
a slide outline. fullDocument is a sequence of slides, each:
# Slide N: [standalone content statement — the key message of this slide]
followed by 3–5 bullet points; fragments are fine, sub-bullets sparingly (one level).
- slide 1 is the executive summary: the main point of the whole deck
- read in sequence, the slide titles tell the whole story
- as many slides as the content needs: a short input makes a short deck
- specific numbers and facts from the input over vague statements — never a number the input does not have
- no greeting, no sign-off

<example name="cost-savings-de">
input: "Präsentation für das Management: Wir haben die Serverkosten um 40% gesenkt durch Migration zu AWS. Das hat 3 Monate gedauert. Wir haben dabei keine Ausfallzeiten gehabt. Die Einsparung beträgt 120.000€ pro Jahr. Im nächsten Schritt wollen wir auch die Datenbank migrieren, das würde nochmal 60.000€ sparen."

output: {"fullDocument": "# Slide 1: AWS-Migration spart 120.000 €/Jahr – Datenbank-Migration brächte weitere 60.000 €\n\n- Serverkosten um 40 % gesenkt\n- Einsparung: 120.000 € pro Jahr\n- Nächster Schritt: Datenbank migrieren, weitere 60.000 € Einsparung\n\n# Slide 2: Migration in 3 Monaten ohne Ausfallzeit\n\n- Dauer: 3 Monate\n- Keine Ausfallzeiten\n\n# Slide 3: Datenbank-Migration als nächster Schritt – weitere 60.000 €\n\n- Geplant: auch die Datenbank migrieren\n- Zusätzliche Einsparung: 60.000 €", "language": "de"}
</example>
</form>`,
}

// buildPromptV3 assembles the v3 system prompt for one form: the shared core,
// that form's section, the output format and the user's style. docType must be
// a valid form (see isValidDocType); the caller resolves "auto" first.
func buildPromptV3(docType, style, relationship, customInstructions, inputText string) (systemPrompt, userMessage string, err error) {
	form, ok := promptV3Forms[docType]
	if !ok {
		return "", "", fmt.Errorf("prompt v3 has no section for document type %q", docType)
	}
	systemPrompt = promptV3Core + "\n\n" + form + "\n\n" + promptV3Output + "\n\n" +
		styleInjection(style, relationship, customInstructions)
	return systemPrompt, inputText, nil
}

// styleInjection is the user's style block, the same for v2 and v3: v2's was
// measured, so v3 keeps its wording.
func styleInjection(style, relationship, customInstructions string) string {
	s := fmt.Sprintf(`<style_injection>
Communication style: %s
Relationship level: %s`, style, relationship)
	if customInstructions != "" {
		s += fmt.Sprintf("\nAdditional instructions: %s", customInstructions)
	}
	return s + "\n</style_injection>"
}
