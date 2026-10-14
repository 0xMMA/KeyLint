# Pyramidize — Requirements

> Canonical requirements for the Pyramidize feature in KeyLint v2.
> Extracted from the original PYRAMIDIZE.md planning doc (2026-03-08).
> This document captures WHAT the feature does. For HOW, see architecture.md and adr/.

---

## User Stories

| ID | As a... | I want to... | So that... |
|----|---------|-------------|---------|
| US-01 | knowledge worker | paste an unstructured email draft and get a pyramidally-structured email with an information-dense subject line | recipients can quickly extract key information |
| US-02 | German/English writer | have my language (or code-switching mix) preserved in the output | I don't need to fix translation artifacts |
| US-03 | user | say in my own words how the text should read ("more formal", "in English") or what it should also say ("mention I'm away Friday") | it reads the way I need — my instruction wins over the author's tone; nothing I did not write is added. There is no style, relationship or audience picker (choice page, [ADR-003 addendum](adr-003-prompt-v3.md)) |
| US-04 | user | pick the document type (email, chat, memo, wiki, presentation) | the output format matches my use case — nothing guesses it for me |
| US-05 | user | provide custom instructions | I can guide the AI without starting over |
| US-06 | user | edit the AI output directly on the canvas | I can make manual tweaks without re-running |
| US-07 | user | give the AI a targeted instruction for the whole canvas or a selected section | I can refine specific parts without losing the rest |
| US-08 | user | see a trace log of every AI operation | I can compare versions and revert to any previous state |
| US-09 | user | copy the result as rich text (HTML) or markdown | I can paste it into Outlook, Teams, or any editor |
| US-10 | user (hotkey flow) | send the result back to the source application | the formatted text goes directly back to my email/document |
| US-11 | user | have the app remember which doc type to use for each source application | I don't re-select every time I come from Outlook or VS Code |
| US-12 | user | cancel any in-progress operation | I'm not blocked waiting for a slow/wrong call |

---

## Functional Requirements

> FR-01 to FR-03 describe Pyramidize v3 ([ADR-003](adr-003-prompt-v3.md),
> 2026-10-11). Until then they described auto-detection, a self-QA score and a
> refine pass; all three are gone.

### FR-01 Document Type: the user picks the form
- Forms: EMAIL, CHAT, MEMO, WIKI, POWERPOINT (presentation). The user always picks one; no model detects it
- Preselection: email, unless an app preset matches the app the shortcut copied from. A preset matches whole words (at least three characters) of the window title, the app's part of the title (after the last " - ") first; the longest match wins
- `auto` (CLI, older clients) resolves the same way without a model call; the CLI has no source app, so there it is email
- An unknown type is an error, not a silent fallback

### FR-02 Generation: one call writes the form
- One prompt per form: a shared core (pyramid, content headers, fidelity, voice, user choices, style) plus that form's section — `prompts_v3.go`
- Output: `{language, fullDocument}` (language first since 2026-10-14, ADR-003 addendum); subject, title and headers are inside `fullDocument`, the subject/title as its first line where the form has one
- Fixed shapes: email always has subject line, greeting as written, bold content headers with bullets and sign-off as written — also for three casual sentences; memo, wiki and presentation keep their shapes too. Chat is the form for a short result
- Information preservation: every fact, name, number, date, request and open question appears; nothing is added (no conclusions, no "no action needed")
- Voice: by default the author's language, code-switching, tone and person stay; the user's own instruction ("auf Englisch", "formeller") wins over that default. Content the user states in the instructions ("mention I'm away Friday") is added where it belongs; nothing else is (2026-10-13)
- MECE rule: headers at same level are mutually exclusive and collectively exhaustive
- Header rule: content headers, NOT process headers
- Priority rule: decisions → actions → context

### FR-03 No self-QA, no refinement
- Exactly one model call per run, for every form. No quality score is asked for or reported; a score a model volunteers is ignored
- Quality is measured outside the product, by the eval suite (deterministic checks and two LLM judges), not by the model grading itself
- The `pyramidize_quality_threshold` setting is inert; nothing reads it

### FR-04 Canvas Model (3-Layer State)
- `originalText` — immutable source
- `pyramidizedText` — snapshot of most recent generation
- `canvasText` — live working surface (user + AI edits)
- State persists across navigation, cleared on window close

### FR-05 Trace Log
- Named entries with canvas snapshots: Original, Pyramidize, AI instructions, Checkpoints
- Peek mode: read-only preview of any snapshot
- Revert: creates new entry (audit trail never destructively modified)

### FR-06 AI Instruction System
- Mode A: Global canvas instruction (full rewrite)
- Mode B: Selection-level splice (context-aware section rewrite, edit mode only)

### FR-07–FR-17
See original spec for: subject/title handling, hover copy, output actions, input sources, source app presets, progress indicator, error handling, empty state, provider badge, keyboard shortcuts, settings tab.

---

## Non-Functional Requirements

| ID | Requirement |
|----|-------------|
| NFR-01 | One call per run, so the wait is that call's; each call is bounded at 120 s |
| NFR-02 | API budget: exactly 1 call per pyramidize |
| NFR-03 | All API failures show actionable messages, canvas state preserved on error |
| NFR-04 | Pyramidize, whole-text changes and splices work with OpenAI, Anthropic Claude, the Claude Code CLI and Ollama |
| NFR-05 | All AI calls through Go backend (no webview -> external API) |
| NFR-06 | Unit-testable Go functions, Angular Vitest specs, prompts as Go constants |

---

## Scoping Decisions

| Decision | Choice |
|----------|--------|
| Pipeline architecture | One call per run for every form; the user picks the form. See [ADR-003](adr-003-prompt-v3.md) (and [ADR-002](adr-002-one-shot-vs-pipeline.md) for why) |
| Prompt format | One XML-structured prompt: shared core + one section per form, all providers |
| Canvas model | 3-layer + trace log |
| Canvas AI | Global rewrite + selection splice |
| Source app presets | Doc type only — remembered from the Form step ("Remember <form> for <app>") or set in Settings › App defaults |
| Out of scope | .pptx export, streaming output, AWS Bedrock, LangChain, per-feature provider selection |
