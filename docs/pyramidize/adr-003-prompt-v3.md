# ADR-003 — Prompt v3: the user picks the form, one call writes it

> Status: accepted, not yet measured · Date: 2026-10-11 · Supersedes [ADR-001](adr-001-pipeline-architecture.md) and decisions 2, 3, 5 and 6 of [ADR-002](adr-002-one-shot-vs-pipeline.md) · Issue: #34 · Roadmap: E3 phase 2 (and the phase 4 cleanup it makes possible)

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
| Core: `<pyramid>` | Main point first, then grouped support; decisions → actions → context; thread history moves to the end, it does not drop out |
| Core: `<headers>` | Content statements, never process labels; same-level headers MECE |
| Core: `<fidelity>` | Every fact, name, number, date, request and open question appears; nothing is added — no conclusions, reasons, or status claims like "no action needed"; what the input leaves open stays open |
| Core: `<voice>` | Never translate; keep the input's code-switching and introduce none; never raise formality or change person; greetings and sign-offs as written |
| Core: `<style>` | Compact but complete; a short message gets a short result; how style, relationship and additional instructions apply |
| Form sections | email (subject line `Decision \| Details \| Actions`, greeting, bold headers + bullets, sign-off), **chat (new)**: no subject, no headers, first sentence carries the point, greeting only if the input has one, may shorten but keeps every fact and ask; memo (`TO \| FROM \| RE` line, one-sentence summary, bold headers); wiki (`# title`, one-sentence summary, `##` sections); presentation (`# Slide N:` titles that tell the story, slide 1 is the summary) |
| Output | `{"fullDocument", "language"}` — subject, title and headers live inside the document, where the user sees them |

The style block (`<style_injection>`) is v2's, word for word.

### Where v3 comes from

The core is **v2-email's rules** — the shipped, measured prompt — generalised to
every form and restructured, not rewritten: the pyramid, the header rule and its
WRONG/RIGHT examples, MECE, decisions → actions → context, "NEVER translate",
"NEVER increase formality or change perspective", "NEVER add conclusions,
interpretations". `TestTheCoreCarriesV2sProvenRules` pins them. Form sections take
their shape rules from the old memo/wiki/powerpoint prompts.

What v3 adds is general, taken from `quality-status.md`'s *Persistent Model
Behaviors* rather than from any sample: no editorial status claims ("Kein
Handlungsbedarf"), a short message stays short (tone drift into headers and
bullets), thread history stays as context (dropped mail history). Nothing was
tuned to an eval sample; no sample was read.

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
  "app" is a window title, so a preset matches when its name appears in the title
  (case-insensitive); of several, the longest name wins. `GetDefaultDocType`
  gives the UI the same answer, so it can show the preselection instead of
  making it out of sight.
- **Volunteered quality fields buy nothing.** Schema enforcement is still off by
  default, so a model may still send `qualityScore`/`qualityFlags`. Under v2
  that bought a refine call (ADR-002's `TestTheOneShotPropertyDependsOnPromptCompliance`).
  Now nothing reads them: `TestVolunteeredQualityFieldsBuyNoSecondCall`.
- **Refusals are explicit.** An unknown document type, or a variant that does not
  exist for the form, is an error before any call; both used to run the email
  prompt (and variant 1) silently. An empty document is an error too, so Send
  back cannot paste nothing over the user's text.
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
   smaller output (no `headers` array) change together.
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
