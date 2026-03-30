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
| US-03 | user | select communication style and relationship level | the tone matches my social context |
| US-04 | user | select the document type or let it auto-detect | the output format matches my use case |
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

### FR-01 Document Type Detection
- Auto-detect document type + language when AUTO is selected
- Supported types: EMAIL, WIKI, POWERPOINT, MEMO
- Detection skipped when user manually selects a type
- If detection fails, default to EMAIL

### FR-02 Foundation Generation
- Generate pyramidally-structured document for the detected/selected type
- Output: `{fullDocument, headers[], language, qualityScore, qualityFlags[]}`
- Subject line embedded as first line of fullDocument
- Information preservation: NO essential content from original may be lost
- MECE rule: headers at same level are mutually exclusive and collectively exhaustive
- Header rule: content headers, NOT process headers
- Priority rule: business impact before technical detail

### FR-03 Self-QA and Optional Refinement
- Foundation call includes self-evaluation: `qualityScore` (0-1) and `qualityFlags[]`
- Flags: `MECE_VIOLATION`, `INFO_LOSS`, `STYLE_MISMATCH`, `SUBJECT_FORMAT`, `FIDELITY_VIOLATION`
- If qualityScore < threshold (default 0.65) -> conditional refinement pass
- Quality threshold configurable in Settings > App Defaults

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
| NFR-01 | Detection < 2s, foundation < 8s, total pipeline < 15s with refinement |
| NFR-02 | API budget: 1-3 calls per pyramidize (detect + foundation + optional refine) |
| NFR-03 | All API failures show actionable messages, canvas state preserved on error |
| NFR-04 | All pipeline steps work with OpenAI, Anthropic Claude, and Ollama |
| NFR-05 | All AI calls through Go backend (no webview -> external API) |
| NFR-06 | Unit-testable Go functions, Angular Vitest specs, prompts as Go constants |

---

## Scoping Decisions

| Decision | Choice |
|----------|--------|
| Pipeline architecture | 2-call adaptive (detect + foundation+self-QA, optional refine) |
| Prompt format | One XML-structured prompt per doc type, all providers |
| Canvas model | 3-layer + trace log |
| Canvas AI | Global rewrite + selection splice |
| Source app presets | Doc type only (style/relationship always default per session) |
| Out of scope | .pptx export, streaming output, AWS Bedrock, LangChain, per-feature provider selection |
