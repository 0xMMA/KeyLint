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
// Mail history in the selection is context, never output (owner decision of
// 2026-10-11, ADR-003 addendum): fidelity covers the author's own message, and
// greeting, sign-off and asks are the author's, not a quoted mail's. The prompt
// states the rule only, not the reasons for it.
//
// General principles only. Nothing here is tuned to an eval sample — the
// examples are the prompt's own, and each one is checked against its input:
// every fact in the output is in the input, and nothing else is.

// promptV3Core is shared by every form.
const promptV3Core = `Restructure the user's text using the Pyramid Principle, in the form described in <form>.
The text is the author's own message, sometimes with mail history: quoted replies, forwarded messages, earlier messages in the thread.

<pyramid>
lead with the main point: the decision, conclusion, request or status the reader needs most.
then the support, grouped: each group backs up the main point, groups do not overlap, together they cover the author's whole message.
order the groups by what the reader needs: decisions → actions → context.
</pyramid>

<headers>
every header is a conclusion or key message — understandable without the text below it. NEVER a process label.

WRONG: "Nächste Schritte", "Background", "Action Items", "Summary"
RIGHT: "Release morgen nicht gefährdet", "MVP blockiert bis Freitag"

same-level headers: mutually exclusive, collectively exhaustive.
</headers>

<fidelity>
every fact, name, number, date, deadline, request and open question in the author's own message appears in the output.
mail history is context only: refer to its facts where the author's message needs them, never contradict it, never reproduce it: no recap, no history section, no quoted originals.
NEVER add conclusions, interpretations, reasons, recommendations or information not in the input.
NEVER add status claims the input does not make, such as "no action needed" or "everything on track".
what the input leaves open stays open: do not resolve it by guessing.
fidelity holds whatever the user chooses in <user_choices>.
</fidelity>

<voice>
by default, keep the author's voice:
- write in the language of the author's message, including its code-switching: where the author mixes languages, mix them the same way; where the author does not, do not introduce another language. NEVER translate on your own.
- NEVER increase formality or change perspective beyond the original: casual stays casual, first person stays first person.
- keep greetings and sign-offs as the author wrote them, where the form has them. never upgrade or invent one.
</voice>

<user_choices>
<style_injection> carries the user's choices: communication style, relationship level and additional instructions. an explicit choice wins over the voice defaults.
- "professional" style with "professional" relationship is the default, not a choice: keep the author's voice.
- any other style or relationship is a choice: adjust register, formality and directness to it, even where that departs from the author's tone — more formal for "authority" or "public", plainer for "casual" or "close".
- additional instructions are the user's own words: follow them over the voice defaults and the form's defaults. "auf Englisch" means write in English; "formeller" means raise the formality.
- a choice changes how the text reads, never what it says: facts, names, numbers, dates and asks stay as in the author's message, and nothing is added.
</user_choices>

<style>
compact but complete: no filler, no courtesy phrases the author's message does not have unless a choice calls for them, nothing the input does not say.
the form's shape is fixed: email, memo, wiki and presentation always take their full shape, however short the author's message. chat is the form for a short result.
</style>`

// promptV3Output is the reply shape every form shares: the document and its
// language. Headers, subject and title live inside the document, where the
// user sees them; nothing reads a separate list of them.
const promptV3Output = `<output_format>
Respond ONLY with valid JSON (no markdown fences, no explanation, no trailing text).
CRITICAL: All double-quote characters inside string values MUST be escaped as \" — this includes quotes in the document text.
{
  "fullDocument": "the restructured text as a single string with \n linebreaks",
  "language": "language of the author's message (ISO 639-1)"
}
</output_format>`

// promptV3Forms holds one section per form, keyed by document type. The
// examples are written to the core's rules: in the email one, the first
// subject has no actions segment because its input asks nothing of anyone.
var promptV3Forms = map[string]string{
	"email": `<form name="email">
an email — always this shape, also for a three-sentence casual mail. fullDocument, in order:
1. subject line (first line, before any greeting)
2. the author's greeting, exactly as written — none if the author's message has none
3. body: at least one bold header (**…**) on its own line, each followed by bullet points
4. the author's sign-off, exactly as written — none if the author's message has none

subject line:
[Decision/Message] | [Key Details] | [Actions/Deadlines]
a mini executive summary: the reader skips the body if this says enough.
include @person in the actions segment when the author asks something of someone.
leave the actions segment out when the author asks nothing of anyone — never fill it with "no action needed".

<examples>
<example name="status-update-de">
input: "Hallo Hans, kurze Info: der Build von gestern Abend ist fehlgeschlagen wegen eines Fehlers im CarService. Ich hab das gefixed und einen neuen Build gestartet. Geht ca. eine Stunde. Das Release für morgen steht trotzdem, wir müssen nicht verschieben."

output: {"fullDocument": "Build-Fehler behoben, Release morgen wie geplant | CarService gefixt, neuer Build läuft (~1h)\n\nHallo Hans,\n\n**Release morgen nicht gefährdet**\n- Build gestern Abend wegen CarService-Fehler fehlgeschlagen\n- Fehler behoben, neuer Build läuft (~1 Stunde)\n- Kein Verschieben nötig", "language": "de"}
</example>

<example name="scheduling-change-en">
input: "hey team, just wanted to update everyone that the Q1 review meeting that was scheduled for next friday has been moved to the following monday at 2pm because the CFO has a conflict. please make sure you have your numbers ready by sunday night so we have time to review. Sarah and Tom need to send me their slides by thursday."

output: {"fullDocument": "Q1 Review Moved to Monday 2pm | Numbers Due Sunday Night | @Sarah @Tom Slides by Thursday\n\nhey team,\n\n**Q1 Review Moved to the Following Monday, 2pm**\n- Was scheduled for next Friday\n- Reason: the CFO has a conflict\n\n**Numbers Due Sunday Night, Slides Due Thursday**\n- All: have your numbers ready, so we have time to review\n- @Sarah, @Tom: send me your slides", "language": "en"}
</example>
</examples>
</form>`,

	"chat": `<form name="chat">
a chat message (Teams, Slack and the like): the form for a short result, read in a few seconds.
- no subject line, no headers, no bold section titles
- the first sentence carries the main point; an ask goes in it or right after it
- separate points may follow as a short list; one point stays one sentence
- greeting and sign-off only if the author's message has them, as written — never add a formula
- shorten the wording freely, but every fact and every ask stays

<example name="deployment-de">
input: "Hi Tom, kurzes Update zum Deployment: hat gestern Abend nicht geklappt, weil das Zertifikat abgelaufen war. Hab ein neues beantragt, sollte bis morgen Mittag da sein. Danach deploye ich direkt. Kannst du bis dahin den Kunden informieren? Danke!"

output: {"fullDocument": "Hi Tom, das Deployment kommt erst mit dem neuen Zertifikat (sollte bis morgen Mittag da sein) – kannst du bis dahin den Kunden informieren?\n\nGestern Abend ist es gescheitert, weil das Zertifikat abgelaufen war. Das neue ist beantragt, danach deploye ich direkt.\n\nDanke!", "language": "de"}
</example>
</form>`,

	"memo": `<form name="memo">
an internal memo. fullDocument, in order:
1. header line (first line, all on one line): **TO:** [recipient] | **FROM:** [sender] | **RE:** [the key message as a statement]
   labels in the language of the author's message (German: **AN:** | **VON:** | **BETREFF:**).
   recipient or sender the input does not name: keep the brackets as a placeholder — never invent a name.
   RE states the message, not the topic. WRONG: "RE: Project Update" RIGHT: "RE: Q2 Launch Slips Two Weeks — Scope Decision Needed by Friday"
2. one sentence that states the key point
3. body: bold headers (**…**) on their own line, each followed by bullet points
no greeting, no sign-off: a memo has neither.

<example name="api-switch-en">
input: "Need to tell the team that we're switching from REST to GraphQL for the new client portal. The decision was made because of performance issues with the current approach - we're making too many round trips. Timeline is 6 weeks starting next monday. Jake and Priya will lead the migration. Everyone else needs to attend a kickoff meeting on friday."

output: {"fullDocument": "**TO:** Team | **FROM:** [sender] | **RE:** We're Switching the Client Portal From REST to GraphQL — Kickoff Friday, Start Monday\n\nWe're switching the new client portal from REST to GraphQL over six weeks from next Monday, because we're making too many round trips.\n\n**Jake and Priya Lead the Migration, Starting Next Monday**\n- Duration: 6 weeks\n- Lead: Jake and Priya\n\n**Everyone Else Attends the Kickoff on Friday**\n- All team members not leading the migration\n\n**Too Many Round Trips Drive the Switch**\n- Performance issues with the current approach\n- Cause: we're making too many round trips", "language": "en"}
</example>
</form>`,

	"wiki": `<form name="wiki">
a wiki page: reference text for any reader, read now or in a year. fullDocument, in order:
1. title (first line): # [precise, specific title that states the topic and its key fact]
2. one sentence that states the key insight
3. sections: ## headers, each followed by bullet points; the most important section first
- code, commands and configuration go in code blocks
- no greeting, no sign-off, no addressing a reader as in a message: they belong to a message, not a page. keep every fact they carried, and the author's person ("we fixed" stays "we fixed").
- keep source references and links from the author's message

<example name="logout-bug-en">
input: "so we had this bug where users were getting logged out randomly. turned out it was because the session token wasnt being refreshed properly. the token expires after 30 mins and we refresh it on every api call but there was a race condition in the refresh logic. fixed it with a mutex. this affects all users on v2.3.1 and below."

output: {"fullDocument": "# Race Condition in Session Token Refresh Logged Users Out at Random (v2.3.1 and Below)\n\nWe fixed random logouts, caused by a race condition in the session token refresh, with a mutex.\n\n## A Mutex Fixes the Race Condition in the Refresh Logic\n- The session token was not refreshed properly\n- Cause: a race condition in the refresh logic\n- We fixed it with a mutex\n\n## All Users on v2.3.1 and Below Are Affected\n- Symptom: users get logged out at random\n\n## The Token Expires After 30 Minutes and Is Refreshed on Every API Call\n- Expiry: 30 minutes\n- We refresh it on every API call", "language": "en"}
</example>
</form>`,

	"powerpoint": `<form name="presentation">
a slide outline. fullDocument is a sequence of slides, each:
# Slide N: [standalone content statement — the key message of this slide]
followed by at most 5 bullet points; fragments are fine, sub-bullets sparingly (one level).
- slide 1 is the executive summary: the main point of the whole deck
- read in sequence, the slide titles tell the whole story
- as many slides as the content needs, never a filler slide
- specific numbers and facts from the author's message over vague statements — never a number the input does not have
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
