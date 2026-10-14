# ADR-003 — Prompt v3: the user picks the form, one call writes it

> Status: accepted; email measured on 2026-10-12 and 2026-10-13, the other forms not yet ([quality-status](quality-status.md)) · Date: 2026-10-11 (addendum on mail history the same day; addendum on the choice page 2026-10-12) · Supersedes [ADR-001](adr-001-pipeline-architecture.md) and decisions 2, 3, 5 and 6 of [ADR-002](adr-002-one-shot-vs-pipeline.md) · Issue: #34 · Roadmap: E3 phase 2 (and the phase 4 cleanup it makes possible)

## Decision

Pyramidize writes the text in the form the **user** picked — email, chat, memo,
wiki or presentation — in **exactly one model call**. No model detects the form,
no self-QA block grades the result, no refine pass rewrites it, and no quality
threshold decides whether one runs. This is the owner's decision of 2026-10-11
(`roadmap.md`, E3), built on ADR-002's finding that for email the two-call
pipeline bought no measured quality and lost documents.

The prompt for every form is **v3**: one shared core plus one section per form
(`internal/features/pyramidize/prompts_v3.go`).

| Part | What it says |
|---|---|
| Core: `<pyramid>` | Main point first, then grouped support; decisions → actions → context. (The first version also said "thread history moves to the end, it does not drop out" — replaced, see the addendum.) |
| Core: `<headers>` | Content statements, never process labels; same-level headers MECE |
| Core: `<fidelity>` | Every fact, name, number, date, request and open question **of the author's own message** appears (addendum); mail history is context only; nothing is added beyond what the user's instructions state (2026-10-13) — no conclusions, reasons, or status claims like "no action needed"; what the input leaves open stays open; holds whatever the user chooses |
| Core: `<voice>` | The default: the language (the author's message's, addendum; and only it, not the language of the prompt's own wording and examples, 2026-10-14) and code-switching (none introduced), tone and person; never raise formality on its own; greetings and sign-offs as written |
| Core: `<user_choices>` | The user's own instructions, when there are any (choice page addendum: no style or relationship any more): they are the user's words and win over the voice and form defaults ("auf Englisch" means English, "formeller" raises formality); content they state is added where it belongs, nothing else is (2026-10-13) |
| Core: `<style>` | Compact but complete, no filler, nothing the input or the user's instructions do not say; the form's shape is fixed — email, memo, wiki and presentation always take their full shape, however short the input; chat is the form for a short result |
| Form sections | email (subject line `Decision \| Details \| Actions`, greeting, bold headers + bullets, sign-off), **chat (new)**: no subject, no headers, first sentence carries the point, greeting only if the author's message has one, may shorten but keeps every fact and ask; memo (`TO \| FROM \| RE` line, one-sentence summary, bold headers); wiki (`# title`, one-sentence summary, `##` sections); presentation (`# Slide N:` titles that tell the story, slide 1 is the summary) |
| Output | `{"language", "fullDocument"}`, language first since 2026-10-14 (addendum below). Subject, title and headers live inside the document, where the user sees them |

The style block (`<style_injection>`) was v2's, word for word; since the choice
page addendum v3 carries only a `<user_instructions>` block, and only when the
user typed something. v2 keeps its block unchanged.

### Where v3 comes from

The core is **v2-email's rules** — the shipped, measured prompt — generalised to
every form and restructured, not rewritten: the pyramid, the header rule and its
WRONG/RIGHT examples, MECE, decisions → actions → context, "NEVER translate",
"NEVER increase formality or change perspective", "NEVER add conclusions,
interpretations". `TestTheCoreCarriesV2sProvenRules` pins them. Form sections take
their shape rules from the old memo/wiki/powerpoint prompts.

What v3 adds is general, taken from `quality-status.md`'s *Persistent Model
Behaviors* rather than from any sample: no editorial status claims ("Kein
Handlungsbedarf") and thread history stays as context (dropped mail history) —
the second replaced by the addendum below: history is context, never output.
Nothing was tuned to an eval sample; no sample was read.

### Two owner decisions (Michael, 2026-10-11, after review)

- **Fixed shapes.** An email always has subject line, greeting as written, bold
  content headers with bullets and sign-off as written — also for a
  three-sentence casual mail; memo, wiki and presentation keep their shapes too.
  The first draft's "a short message gets a short result" contradicted that (and
  the eval's structure check, which wants headers) and went. "No padding" stays,
  meaning no invented content and no filler. **Chat is the form for a short
  result.** The structure check is unchanged, so no `checksVersion` bump.
- **An explicit choice wins over the author-voice defaults; content never
  changes.** Without a choice the author's language, code-switching and tone
  stay. A style or relationship implying more formality, or an instruction like
  "auf Englisch" or "formeller", is followed — never over fidelity. Because the
  UI and CLI always send professional/professional, that pair is defined as the
  default, not as a choice; phase 3 replaces the relationship levels anyway.

### Addendum — mail history is context, never output (Michael, 2026-10-11)

Mail history in the selected text — quoted replies, forwarded mails, the thread
above or below the author's own text — is **context only**. The user selected
it (often with Ctrl+A, sometimes without meaning to) so the model can
understand the message and refer to it. **The output never reproduces earlier
messages:** no recap, no history section, no quoted originals. Facts from the
history may be referred to where the author's message needs them, and must not
be contradicted. The accepted output of `email-dataquality-reply-to-feedback`
shows the intended history behaviour: a reply that answers, without restating
the mails it answers. It is not a fidelity model — it adds a closing ask and a
thank-you the author did not write.

This replaces the first version's "earlier messages in a thread are context:
they move to the end, they do not drop out", which produced exactly that recap
(quality-status, "prompt v3 against v2"). In the prompt:

- The core's opening says the text is the author's own message, sometimes with
  mail history; `<fidelity>` covers the author's own message and carries the
  rule in one line: *"mail history is context only: refer to its facts where
  the author's message needs them, never contradict it, never reproduce it: no
  recap, no history section, no quoted originals."* The pyramid's groups cover
  "the author's whole message", not "the whole input".
- What follows from it, stated as general principles: the reply's language is
  the author's message's; the email's greeting, sign-off and `@person` asks,
  and the chat's greeting and sign-off, are the author's, not a quoted mail's;
  so are the asks a user choice keeps, the courtesy phrases, the memo's label
  language, the wiki's links and the presentation's facts. `<fidelity>`'s
  "information not in the input" stays as it is: a reference to the history is
  not added information.
- The prompt states the rule, not the reasons for it.
  `TestMailHistoryIsContextNeverOutput` pins it; v2 is untouched.

The eval follows, so it does not punish the intended behaviour:

- **Samples mark the history** with `<!-- history -->` … `<!-- /history -->`
  lines inside the raw input (format: `eval_samples.go`). Two of the 13 carry
  history and are marked: `email-diagnose-update` (thread below) and
  `email-dataquality-reply-to-feedback` (two quoted mails above, with the
  author's labels for them). The marker lines are stripped before anything
  reads the input, so Pyramidize and the judges get byte for byte what the user
  would select (`TestTheMarkedSamplesSendWhatTheUserSelected` pins the hashes).
- **Info coverage counts only the author's message**; the hallucination check
  still reads the whole input, so a fact referred to from the history is not
  "invented", but takes its ratio over output terms the history does not hold,
  so pasted history cannot dilute it (it did: appending the history to the
  accepted reply-to-feedback output raised its deterministic score). A new
  check, **`no_history_reproduction`**, for samples with history only, fails a
  copy: 12 or more consecutive words shared with the history, or 10 % or more
  of the history's 8-word windows found verbatim in the output (score = 1 minus
  that share). A reference keeps a name or a phrase; a quote keeps sentences.
  `checksVersion` goes from 1 (never recorded) to **2**.
- **The judges are told** that history is context, that the candidate must not
  reproduce it, and that leaving it out is no completeness flaw; completeness
  asks for the author's own message, tone the language of the author's
  message. The judge instrument — system prompt, user-message template, schema
  and output limit — is now recorded as `judgePromptHash` and is part of the
  `configKey`.
- So **every earlier baseline reads "not comparable"** with a run on this
  instrument, the v3 and v2 references of 2026-10-11 included. Both needed new
  runs: v3 ran on 2026-10-12 (and again after the choice page on 2026-10-13),
  v2 has not been rerun.

v2 is byte-identical through both: `TestV2IsKeptVerbatim` pins sha256 hashes
of its system prompt, computed from main's file built on its own.

**The examples were fixed, because they taught the faults the rules forbid.**
v2's first email example put *"Keine Aktion erforderlich"* into a subject whose
input said nothing of the kind — the same editorial addition quality-status lists
as persistent. Its English example used a German header word (*"statt"*) in an
all-English input — code-switching the input never had — added "EOD" to a
deadline, and used "Deadlines", a process label, as a header. The old
presentation, memo and wiki examples invented facts outright (a blue-green
deployment, "January–March", 100 % uptime, "details follow separately",
"v2.4.0+ includes the fix"). Every v3 example is checked against its input: each
fact in the output is in the input, and nothing else is.
`TestV3ExamplesAreValidReplies` checks they parse in the v3 shape.

## The run path

- **One call, for every form.** `TestEveryFormCostsOneCall` counts client calls
  through the real `Pyramidize` entry point, for every form, "auto" and both
  variants.
- **"auto" is a lookup, not a call.** `docType: "auto"` (or empty) resolves to
  the app preset for the captured source app — the presets in Settings › App
  defaults were stored but never applied until now — else **email**. The captured
  "app" is a window title, and Windows titles end in the app ("Inbox - Outlook",
  "Report.docx – Word"). A preset name matches whole words only, ignoring case,
  and needs three characters ("Mail" must not hit "Gmail" or "Email
  draft.docx"); the app segment after the last " - "/" – "/" — " is searched
  first, the document part only when it matches nothing (a browser tab names
  its web app there); the longest name wins. The document-part fallback still
  lets a Teams-only preset match an Outlook mail titled "Microsoft Teams
  rollout"; a preset for Outlook takes precedence over it. `GetSourcePreset`
  (it replaced `GetDefaultDocType` with the choice page) gives the UI the same
  answer, so it can show the preselection instead of making it out of sight.
- **Volunteered quality fields buy nothing.** Schema enforcement is still off by
  default, so a model may still send `qualityScore`/`qualityFlags`. Under v2
  that bought a refine call (ADR-002's `TestTheOneShotPropertyDependsOnPromptCompliance`).
  Now nothing reads them: `TestVolunteeredQualityFieldsBuyNoSecondCall`.
- **Refusals are explicit.** An unknown document type, or a variant that does not
  exist for the form, is an error before any call; both used to run the email
  prompt (and variant 1) silently. A model reply without a document is an error
  too, so Send back cannot paste nothing over the user's text. An unreadable or
  empty reply is logged redacted (it may hold the user's text) and the UI gets
  a plain message — the error used to carry the whole reply; the same now holds
  for `RefineGlobal` and `Splice`.
- `RefineGlobal` and `Splice` (user-driven editing) are unchanged, apart from
  one line in the whole-text prompt that no longer assumes a subject line (a chat
  has none).

## Prompt variants

| `--variant` / `PromptVariant` | Prompt | Forms |
|---|---|---|
| 0 (default) = 3 | v3 | all five |
| 2 | v2 email, verbatim (`TestV2IsKeptVerbatim`), with its three-field schema | email only — an error for any other form |
| 1 | **removed** — an error | — |

**Why v1 went:** v1 was the pipeline arm of ADR-002 — self-QA plus the refine
pass it fed. With both gone it would be neither the pipeline that was measured
nor a prompt anyone ships; keeping it would only preserve a number that no
longer refers to anything. ADR-002's results stand as history.

**Why the memo, wiki and powerpoint prompts went:** without the self-QA block
they had no output format at all (it lived in that block), and their examples
invented facts. Their shape rules live on in v3's form sections. The roadmap's
"v3 against today's pipeline" comparison for those forms can still be run
against the `v4.7.1-beta` tag once samples exist; nothing in the eval harness
ran those forms anyway.

## What was removed

- `prompts_detect.go`, `prompts_selfqa.go`, `prompts_refine.go`,
  `prompts_email.go` (v1 and the old dispatcher), `prompts_memo.go`,
  `prompts_wiki.go`, `prompts_powerpoint.go`, `source_app_map.go`
  (`LookupDocType`, never called; app presets replace it).
- The detect and refine steps, `detectSchema`, `documentSchema`, and the
  `detectResult`/`foundationResult`/`refineResult` types.
- From `PyramidizeResult` (and so from the bindings and `-pyramidize --json`):
  `headers`, `qualityScore`, `qualityFlags`, `appliedRefinement`,
  `refinementWarning`, `detectedType`, `detectedLang`, `detectedConfidence`.
  `--json` is now `{documentType, language, fullDocument}` — ADR-002 decision 6,
  "a documented output that lies", is closed.
- The `GetQualityThreshold`/`SetQualityThreshold` bindings. Nothing called them
  since the control left the UI, and the bindings were regenerated anyway.
  **Kept:** the `pyramidize_quality_threshold` settings field (old settings files
  keep loading, no settings-model churn) and its recording in eval
  `summary.json`, because `qualityThreshold` is part of the eval `configKey`:
  dropping it would make every earlier baseline "not comparable" for a field
  that changed nothing. It is inert.
- In the UI (minimal, the full rework is phase 3): "Detect automatically" left the
  document-type picker, Chat joined it, email is preselected and a shortcut
  capture preselects the app preset; the "Detected: …" hint, the refinement
  warning and "Step 1/2" went with the steps behind them.
- The CLI's `-type` default is `email` (a command line has no source app, so
  "auto" would be email anyway; `auto` is still accepted).

## What is measured next

1. **v3 email against v2 email**, three runs each, with the two-judge instrument
   (E3 phase 1, Claude Code CLI): `./scripts/eval.sh --runs 3` (v3) and
   `./scripts/eval.sh --variant 2 --runs 3` (v2). **v3 must not regress against
   v2** — any metric's interval entirely below v2's is a regression. The variant
   is part of the `configKey`, so `--compare` answers "not comparable" for this
   pair by design; the verdict is the same interval rule applied by hand, as
   ADR-002 did, and reported as such.
   The comparison is v3 as a package: the core, the fixed examples and the
   smaller output (no `headers` array) change together. Each Pyramidize run now
   records `promptHash` in `summary.json` (reported next to a verdict, not in
   the `configKey`, as for the Fix suite), so v3 iterations can be told apart.
2. **Chat, memo, wiki, presentation wait for real samples.** The suite is 13
   German business emails; the other forms have no eval coverage and v3's
   sections for them are unmeasured. Anonymised real texts from Michael are the
   precondition (roadmap E3 phase 1).
3. Then the model question on v3 (Sonnet 5.5, Opus 5.5, maybe Haiku 5.5), and
   schema enforcement on v3 with and without (E3, still open from #47).

## What would change this decision

- v3 email regressing against v2 on either judge or the deterministic score:
  fix v3 on general principles and measure again; v2 stays selectable until then.
- Samples for another form showing its v3 section misses something the old
  pipeline prompt got right — the old prompts are in the `v4.7.1-beta` tag.

## Addendum — the choice page: the form and the user's instructions, nothing else (Michael, 2026-10-12)

The Pyramidize page became **Text → Form → Result** (roadmap E3 phase 3,
decided on the mockup the same day). The Form step is five large buttons —
email, chat, memo, wiki, presentation, keys 1–5 — one field, "Instructions
(optional)", and "Pyramidize as <form>" (Enter). **There is no tone, style,
relationship or audience control anywhere in Pyramidize:** the text already
shows its reader (greeting, @names, mail history), the prompt keeps the
author's voice by default, and the instructions field covers the rest ("more
formal", "in English"). The options column went with its per-session provider
switch and model and effort pickers: provider, model and effort come from
Settings only, and the Form step names them with a link there.

**In the prompt**, `<user_choices>` shrinks to the user's instructions. As
amended by Michael's decision of 2026-10-13 (below):

```
<user_choices>
<user_instructions> holds the user's own instructions for this text, when they gave any. they win over the voice defaults and the form's defaults.
- without instructions, keep the author's voice as <voice> says.
- instructions are the user's own words: follow them. "auf Englisch" means write in English; "formeller" means raise the formality.
- content the user states in their instructions is added, placed where it belongs in the pyramid. everything else keeps the author's facts, names, numbers, dates and asks as written, and nothing else is added.
</user_choices>
```

- The "professional/professional is the default, not a choice" line and the
  style/relationship rule (more formal for authority or public, plainer for
  casual or close) are gone. So is v3's `<style_injection>`: the system prompt
  ends with `<user_instructions>` holding the trimmed instructions, and with no
  block at all when the field is empty. General principles only; nothing
  tuned to an eval sample.
  `TestUserChoicesAreTheUsersInstructionsOnly` and
  `TestInstructionsReachTheV3Prompt` pin it, `TestInstructionsReachTheModel`
  pins it through the real `Pyramidize`.
- **v2 is byte-identical.** It still has its style block, and every v2 run now
  sends it the professional/professional it was measured with
  (`TestV2IsKeptVerbatim` keeps its hashes and checks what a run sends).
- `communicationStyle` and `relationshipLevel` left `PyramidizeRequest` and
  `RefineGlobalRequest` (and the bindings). No settings field ever held them.
  "Change the whole text" kept "the communication style and relationship
  level" of a page that no longer has them; it now keeps the document type and
  the voice of the current canvas.
- The CLI's `--style` and `--relationship` are accepted and ignored, so a
  script that passes them keeps running; `--instructions` is the CLI's version
  of the field.
- The prompt text changed, so v3's `promptHash` changed: a small control run of
  v3 email on the current instrument follows (roadmap E3), not part of this
  change.

**On the page**, what the brief and the mockup settled:

- The shortcut's text skips Text and opens on Form; pasted or typed text starts
  on Text. Only a shortcut capture makes text "from an app": pasted text, or
  text emptied and retyped, is not, so it gets no "Remember" box and no Send
  back.
- Preselection: the app's remembered form, else email. The card the page
  preselected for an app carries a tag ("Default", or "Remembered for <app>");
  pasted text tags none, as in the mockup.
- "Remember <form> for <app>" is unticked, unticks again on every other pick,
  and writes the preset when the user runs with it ticked, not on tick. When
  the picked form already is the app's remembered one, "<Form> is remembered
  for <app>." with Forget takes its place. `GetSourcePreset` names the app to
  remember under: the matched preset's own name (so remembering another form
  updates it and Forget deletes it), else the app segment of the window title
  ("Outlook" for "Inbox - Outlook"), which is what the matcher searches first.
- Running again always starts from the original text and adds a result;
  earlier results stay whole (with their edits) and selectable as version
  chips, and the history keeps every snapshot. Cancel goes back to Form, and a
  reply that arrives after a cancel is dropped.

### Instructions may add content (Michael, 2026-10-13)

What the user writes explicitly in the Instructions field ("mention I'm away
Friday") goes into the text, placed where it belongs in the pyramid; the model
still invents nothing on its own, and the author's facts, names, numbers,
dates and asks stay as written. In the prompt: `<user_choices>` as quoted
above (the first version said "a choice changes how the text reads, never what
it says … and nothing is added"), and `<fidelity>`'s never-add lines leave room
for exactly that — "information not in the input or the user's instructions",
"status claims that neither the input nor the user's instructions make",
"fidelity holds whatever the user chooses in <user_choices>: only content the
user's instructions state is added". `<style>` no longer speaks of "a choice"
for courtesy phrases but of the user's instructions.
`TestInstructionsMayAddWhatTheUserStates` pins it. v2 is byte-identical. The
v3 email control run happens after this change (roadmap E3 phase 3).

### Review round (PR #130)

- **Which name an app is remembered under** follows real Windows titles. A
  title is split at " - ", " – ", " — " and " | " (new Teams: "Chat | Max
  Mustermann | Microsoft Teams"); the app is the last segment, and where that
  is a browser (Chrome, Chromium, Edge, Firefox, Brave, Opera, Vivaldi, Safari
  — a short list, compared ignoring case and Edge's zero-width space) the web
  app before it ("… - Gmail - Google Chrome" → "Gmail"). That segment is what
  the matcher searches first and what Remember stores, so the two agree;
  after it the matcher searches the document part of an app's title, but in a
  browser only the browser's name, never another page's title. No Remember
  box where nothing safe can be stored: a title without a separator (it may
  be a person or a document), a name under three characters (the matcher
  ignores those) or over 40 (more likely a subject than an app). Not
  recognised: a browser profile segment between web app and browser (Edge's
  "… - Gmail - Work - Microsoft Edge"); the roadmap's Windows check stays open.
  `GetSourcePreset` now takes the session's title, so it answers for the
  session even when the backend's capture is newer.
- **Preset names compare trimmed and case-folded** on save, match and delete
  (Go, and Settings › App defaults trims too); the page asks the backend
  again after Remember and Forget instead of assuming.
- **One call at a time:** a new Pyramidize, RefineGlobal or Splice cancels
  the one still in flight (they share one cancel); the page marks a run
  before it awaits anything, so a double click or held Enter starts one; a
  shortcut or paste that replaces the session cancels a run in flight.
- **The source app belongs to the session**, not to the text: set when the
  shortcut starts a session, cleared only by a new one (a shortcut replace or
  a paste replace, which now asks too). Each result keeps the original and
  the source it was made from: whole-text changes and splices use that
  original, and Send back stays offered for it — but only pastes while the
  backend's captured window still is the session's ("The shortcut was used in
  another window since — copy the text instead" otherwise).
- A run in flight, its label, errors and Retry live with the session, so
  leaving and coming back shows the working card, and the result when it
  lands. An error shows on its own step and clears on a step, form or
  instructions change; Retry of a run uses what is picked now.
- Reverting a history snapshot writes it into the version it was taken of,
  and shows that version.
- CLI: passing `--style` or `--relationship` prints one line on stderr that
  they are ignored and that `--instructions` is the way.

## Addendum — the reply names its language first (2026-10-14)

**English input came back in German.** The website's demo notes were English,
and Pyramidize returned German in 6 of 7 calls on v3. v2 did the same in 3 of
3 (Sonnet 5.5 through the CLI). Some replies were German throughout. Others
had German subject and headers over English bullets. The main suite could not
see it: its 13 samples are German.

One diagnostic call ruled out the user's own `~/.claude/CLAUDE.md`, which is
written in German. That left the prompt. Its reply named the language only
after the document. Its `<headers>` examples and the `<user_choices>` examples
are German, and so are some of the form examples.

**The fix:**
- **Language first.** The reply's language is now its first field, "decided
  first: the document is written in it". This applies to the output format,
  every example and `documentSchemaV3`.
- **A new line in `<voice>`.** It says the language comes from the author's
  message alone, and that the languages of the instructions and examples tell
  nothing.

Both are general principles, not tuned to any sample. On the same notes, 0 of
6 calls came back in German, and a German eval sample stayed German.

**The guards:**
- `TestV3NamesTheLanguageBeforeTheDocument` pins the order.
- `TestEvalLanguage` measures it, with 8 samples in all five forms, English and
  German.
- `checkLanguage` gives each line one vote, counted by function words. Its
  fixtures are the replies from that day. It passes every accepted output in
  the main suite, including `email-software-deployment`, a German mail with a
  pasted English README section.

v2 stays byte-identical, and so it keeps the fault. It is a comparison baseline
only.
