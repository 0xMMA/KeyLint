# Pyramidize — Requirements & Engineering Plan

> Mini project plan for the Pyramidize feature rebuild in KeyLint v2 (Go + Wails + Angular).
> Check off milestones independently: Requirements phase first, then Engineering phase.
> Last updated: 2026-03-08 (post second interactive requirements session — all gaps resolved, REQ-05 signed off)

---

## Status Board

### ✅ Requirements Phase
- [x] **REQ-01** User stories finalized
- [x] **REQ-02** Functional requirements reviewed & agreed
- [x] **REQ-03** Non-functional requirements reviewed & agreed
- [x] **REQ-04** Scoping decisions made (see §R.4)
- [x] **REQ-05** SOTA prompting approach reviewed & signed off
- [x] **REQ-06** Architecture decision recorded (2-call adaptive)

### 🔧 Engineering Phase
- [x] **ENG-01** Go backend — new `pyramidize` feature package scaffolded
- [x] **ENG-02** Go backend — prompt templates (detect + 4 doc types + global refine + splice refine)
- [x] **ENG-03** Go backend — `Pyramidize`, `RefineGlobal`, `Splice`, `CancelOperation`, `SendBack` RPC methods + tests
- [x] **ENG-04** Go backend — preset storage + quality threshold in settings struct + `AppDefaults` RPC methods
- [x] **ENG-05** Wire DI + bindings regenerated
- [x] **ENG-06** Angular — Settings: new "App Defaults" tab
- [x] **ENG-07** Angular — Pyramidize page: full layout, state model, trace log
- [x] **ENG-08** Angular — Canvas: textarea + preview toggle + hover copy overlay
- [x] **ENG-09** Angular — AI instruction bar (global) + selection bubble + keyboard shortcuts
- [x] **ENG-10** Angular — Vitest specs passing (0 failures) — 108/108
- [x] **ENG-11** Go tests passing (`go test ./internal/...`) — 47 pyramidize + all feature tests
- [x] **ENG-12** Binary builds cleanly (`go build -o bin/KeyLint .`)
- [ ] **ENG-13** Manual E2E verified (all scenarios in §I.7)

---

## Phase R: Requirements Engineering

### R.1 User Stories

| ID | As a… | I want to… | So that… |
|----|-------|-----------|---------|
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

### R.2 Functional Requirements

#### FR-01 Document Type Detection
- System SHALL auto-detect document type + language when AUTO is selected
- Returns `{type, language, confidence}` — confidence shown as indicator near selector
- Supported types: EMAIL, WIKI, POWERPOINT, MEMO
- **Detection is skipped** when user manually selects a type (saves one API call)
- If detection fails → default to EMAIL, user can correct

#### FR-02 Foundation Generation (core pyramidize call)
- System SHALL generate a pyramidally-structured document for the detected/selected type
- Output fields: `subject/title` (embedded as first line), `headers[]`, `fullDocument`, `language`, `qualityScore`, `qualityFlags[]`
- **Information preservation**: NO essential content from original may be lost
- **MECE rule**: headers at same level are mutually exclusive and collectively exhaustive
- **Email subject format**: `[Main] | [Details/Status] | [Actions/Deadlines] | [@Persons if needed]`
- **Header rule**: content headers ("Login failure on server A"), NOT process headers ("Next Steps")
- **Priority rule**: business impact before technical detail
- Communication style and relationship level injected into prompt
- Custom instructions appended to prompt (not persisted between sessions — security)

#### FR-03 Self-QA and Optional Refinement
- Foundation call includes self-evaluation: `qualityScore` (0–1) and `qualityFlags[]`
- Flag values: `MECE_VIOLATION`, `INFO_LOSS`, `STYLE_MISMATCH`, `SUBJECT_FORMAT`
- If `qualityScore < 0.65` → trigger a targeted second API call with specific flag remediation
- If refinement also fails threshold → return best result with a warning badge in the UI

#### FR-04 Canvas Model — 3-Layer State
The working surface has three distinct layers:
```
originalText       — immutable source (what was pasted or grabbed via hotkey)
pyramidizedText    — snapshot of the most recent foundation generation (checkpoint)
canvasText         — live working surface (starts as pyramidizedText, user + AI edits it)
```
- Canvas state (all three layers + trace log) persists in module-level variables
- State is preserved across navigation (Settings, Fix, back to Pyramidize)
- State is cleared only when the app window is closed (minimises to tray = preserved)

#### FR-05 Trace Log
- Every meaningful state change creates a named entry with a canvas snapshot:
  - `"Original"` — when text first arrives (paste or hotkey)
  - `"Pyramidize"` — after each foundation generation
  - `"AI: [instruction text]"` — after each global or selection AI edit
  - `"Checkpoint"` — manually triggered by user via [+] button in trace log panel header
- **Peek mode**: clicking any entry shows a read-only preview of that canvas snapshot
- **Revert**: "Revert to here" button restores that snapshot AND creates a new trace entry `"Reverted to: [entry name]"` (audit trail is never destructively modified)
- Trace log panel is collapsible (right side), collapsed by default

#### FR-06 AI Instruction System — Two Modes

**Mode A: Global canvas instruction** (instruction bar, fixed at bottom of canvas)
- User types instruction → Ctrl+Enter or [Apply] → sends `{fullCanvas, originalText, instruction, docType, style, relationship}`
- AI returns entire new canvas → replaces canvasText
- Creates trace entry `"AI: [instruction]"`
- [Apply] button and Ctrl+Enter are **disabled** when the instruction field is empty

**Mode B: Selection-level instruction** (floating bubble on text selection — EDIT mode only)
- User selects text in the **textarea (edit mode only)** → floating "✨ Ask AI" button appears
- User types instruction → Enter or [Apply] → sends `{fullCanvas, originalText, selectedText, instruction}`
- AI returns rewritten version of the selected section only
- System splices the rewritten section back into canvasText at the selection bounds
- Creates trace entry `"AI: [instruction] (selection)"`
- Selection bubble does **not** appear in Preview mode (preview is read-only for review and per-paragraph copy)
- Note: context-aware splice may need iteration/testing; if section coherence suffers, escalate to Mode A

#### FR-07 Subject / Title Field
- The subject line (email) or title (wiki/memo/ppt) is embedded as the first line of the canvas
- In both edit mode and preview mode, each line and each paragraph/section block shows a **hover copy icon** (grey clipboard icon appears at right edge on mouse hover)
- Clicking the icon copies that line or block to clipboard as **plain text** (markdown symbols stripped) — this is intentional for email subjects pasted into Outlook/Teams

#### FR-08 Hover Copy Implementation
- **Preview mode (rendered HTML)**: per-paragraph/section `<span>` with CSS-triggered copy button — simple DOM
- **Edit mode (raw textarea)**: mouse-position overlay tracks which line the cursor is over; a floating copy button is positioned at the right edge of that line

#### FR-09 Output Actions
- **Always available**: `Copy as Markdown`, `Copy as Rich Text` (HTML + plain MIME types)
- **Hotkey flow only**: `Send back to [source app]` — button is hidden (not just disabled) in manual-paste flow
- `Copy as Rich Text`: converts canvasText markdown → HTML **client-side** using `marked.js` (same library used for Preview rendering), then writes `ClipboardItem` with `text/html` + `text/plain` MIME types. If `navigator.clipboard.write` is unavailable (WebKit2GTK limitation on Linux), falls back to Go-side conversion at implementation time.
- `Send back to [App]`: calls `SendBack(text)` RPC — Go (a) writes to clipboard, (b) restores focus to the captured source-app window (`SetForegroundWindow` on Windows / `xdotool windowfocus` on Linux), (c) simulates Ctrl+V to paste

#### FR-10 Input Sources
- **Manual paste**: user opens page, pastes into Original textarea (Ctrl+V), or clicks "📋 Paste from clipboard" button in empty state placeholder (small, low-emphasis button)
- **Original textarea** is always editable — user can refine the original draft at any time. The trace log's "Original" entry records the first-arrival snapshot; a user can always revert to it.
- **Hotkey**: immediately before clipboard is grabbed, Go captures the foreground window handle and process name (Windows: `GetForegroundWindow()` + `GetWindowText()`; Linux: `xdotool getactivewindow getwindowname` — best-effort, empty string if xdotool absent). The captured handle is stored in the service for `SendBack()`. Text + source app name → Pyramidize view.
- **Hotkey + in-progress session conflict**: if Original or Canvas already has content when a hotkey fires, show confirmation dialog: "New text received from hotkey. Replace current session?" / [Replace] [Keep editing]
- **Re-pyramidize confirmation**: if user clicks Pyramidize while canvasText is non-empty, show: "Re-pyramidize from original? Your canvas edits will be saved to the trace log." / [Re-pyramidize] [Cancel]. On confirm: auto-create trace entry `"Canvas (saved)"` with current canvasText snapshot, then run pyramidize from `originalText`.

#### FR-11 Source App Presets — Full System

**Auto-detection (built-in mapping)**
When text arrives via hotkey, the source app name is captured. The system maps known apps to doc types:
- Outlook, Thunderbird, Apple Mail → EMAIL
- Microsoft Teams, Slack → EMAIL
- VS Code, Notepad++, Sublime Text → MEMO
- Microsoft Word, Google Docs → MEMO (fallback)
- PowerPoint, Keynote → POWERPOINT

**First-time app eyecatcher**
- When a source app is seen for the first time (no preset saved yet), the doc type selector is highlighted with an eyecatcher animation + label:
  `"Detected: Microsoft Outlook — suggests Email. [Set as default for Outlook]"`
- Clicking "Set as default" saves `{sourceApp → docType}` — saves **doc type only** (style + relationship always use smart defaults per session)
- On subsequent launches from that app, doc type is pre-selected silently; no eyecatcher

**Preset management**
- `"Manage presets"` link/button at bottom of settings panel → navigates to Settings › App Defaults tab
- Settings › App Defaults tab lists all saved presets with edit ✏️ and delete 🗑️ per row
- User can manually add a preset entry if desired

#### FR-12 Progress & Step Indicator
- Steps shown inline where the Pyramidize button was (or in the canvas header area):
  - If AUTO doc type: `"Step 1/2: Detecting document type…"` → `"Step 2/2: Structuring…"`
  - If type manually selected: `"Step 1/1: Structuring with Pyramid Principle…"`
- Cancel button always visible during any in-progress operation (Pyramidize, RefineGlobal, or Splice)
- Clicking Cancel calls the generic `CancelOperation()` RPC — cancels whichever operation is in flight
- If cancelled: progress indicator disappears, no error shown, canvas state unchanged

#### FR-13 Error Handling
- Errors displayed inline below the step indicator area
- Format: `❌ Step N/N failed: [reason]. [Retry] [Change provider → Settings]`
- Canvas is not cleared on error — original text and any prior canvas state are preserved
- **Retry** re-reads the **current UI state** at click time (doc type, style, relationship as set in the selectors right now). If user changed doc type from AUTO to MEMO during the wait, retry uses MEMO. If detection already succeeded in the failed run, it is skipped on retry.

#### FR-14 Empty State
- Original area shows ghost/placeholder text: a greyed example of a pyramidized email
- Placeholder prompt: "Paste or type your draft here, or use the global hotkey"
- "📋 Paste from clipboard" button inside placeholder (small, low-emphasis)
- Pyramidize button is disabled until Original area has content
- If no API provider configured: amber inline banner `"⚠️ No AI provider configured. [Open Settings →] [Dismiss]"`
- The [Dismiss] closes the banner for the current page visit only — it reappears on the next navigation to Pyramidize. The banner disappears permanently only once a key is configured.

#### FR-15 Provider Badge & Detection Indicator
- Below the Pyramidize button in the settings panel: a muted badge showing `"⚫ [Model] · [Provider]"`
- Clicking the badge navigates to Settings › Providers
- Badge text updates reactively when provider is changed in Settings
- **Detection indicator**: after AUTO detection runs, a small badge appears below the doc type selector showing only the detected type (e.g., `● EMAIL`) — no confidence percentage (user already knows what they sent). Badge disappears when user manually selects a type.

#### FR-16 Keyboard Shortcuts
- `Ctrl+Enter` when focus is in Original textarea → triggers Pyramidize
- `Ctrl+Enter` when focus is in global instruction bar → applies global AI instruction
- `Enter` when focus is in selection bubble → applies selection AI instruction
- The Pyramidize button and Apply buttons show grey shortcut annotations (e.g., `Ctrl+↵`)

#### FR-17 Settings — App Defaults Tab
- New tab in Settings alongside existing tabs: `[Providers] [App Defaults] [About]`
- Lists all saved source-app presets: `{sourceApp, docType}`
- Edit button opens inline edit row; delete removes preset
- Empty state: `"No app defaults saved yet. Use Pyramidize with the global hotkey to detect apps automatically."`

---

### R.3 Non-Functional Requirements

#### NFR-01 Latency
- Detection call: < 2 s
- Foundation generation: < 8 s for ≤ 300-word input
- Total pipeline: < 12 s (< 15 s with refinement pass)
- Global canvas AI instruction: < 8 s
- Selection splice AI: < 5 s (smaller prompt surface)
- UI SHALL show step indicator during all operations

#### NFR-02 API Call Budget
- **Minimum**: 1 call (user selects type + quality passes threshold)
- **Typical**: 2 calls (auto-detect + foundation+self-QA)
- **Maximum**: 3 calls (auto-detect + foundation + refinement)
- Canvas AI instructions: 1 additional call per instruction (not part of pyramidize pipeline)

#### NFR-03 Error Handling
- All API failures surface actionable error messages (not raw stack traces)
- Canvas state always preserved on error
- 30 s default timeout per call; configurable later

#### NFR-04 Provider Compatibility
- All pipeline steps MUST work with: OpenAI (`gpt-4o-mini`), Anthropic Claude (`claude-haiku-4-5-20251001`), Ollama (`llama3.2`)
- JSON parsing SHALL strip markdown code fences and trim whitespace before unmarshal
- Ollama note: complex JSON schemas may degrade with smaller models — detection prompt is intentionally minimal

#### NFR-05 Architecture Compliance
- All AI calls through Go backend (no direct webview → external API calls)
- Go structs define the RPC contract; `wails3 generate bindings` generates Angular types
- Canvas + trace state stored in Angular module-level `let` variables (not component props)

#### NFR-06 Testability
- Each Go function unit-testable via table-driven tests (mock HTTP, no real API calls)
- Angular Vitest specs: state model, tab switching, trace log entries, keyboard shortcuts, mock WailsService
- Prompt templates stored as Go `const` strings for easy review

---

### R.4 Scoping Decisions (resolved)

| Decision | Choice |
|----------|--------|
| UI: replace or coexist | **Replace** `/enhance` route with Pyramidize. Grammar fix remains silent hotkey only. |
| Document types | **All 4**: EMAIL, WIKI, MEMO, POWERPOINT |
| Pipeline architecture | **2-call adaptive**: detect (if AUTO) + foundation+self-QA. Optional 3rd call if quality < threshold |
| Quality threshold | **Exposed in App Defaults tab** as number input (0–1, step 0.05, default 0.65). Stored in `Settings.PyramidizeQualityThreshold`. |
| Prompt format | **One XML-structured prompt** (`<role>`, `<task>`, `<constraints>`, `<examples>`, `<output_format>`) for all providers. XML tags are ignored by OpenAI/Ollama but don't break them. |
| Canvas model | **3-layer** (original / pyramidized snapshot / canvas) + trace log |
| Canvas AI scope | **Full canvas** (global instruction bar) + **selection-level** (context-aware splice — EDIT mode only) |
| Canvas editor | **Textarea + preview toggle** (`marked.js` for preview rendering). Hover copy (plain text) in both modes. |
| Trace log | **Collapsible right panel**. Entries on AI ops + explicit checkpoint + auto "Canvas (saved)" before re-pyramidize. Revert = new entry. |
| Settings persistence | **No** — always smart defaults per session. Presets for source-app doc type only. |
| Source app presets | **Full MVP**: auto-detect + eyecatcher + Settings App Defaults tab |
| Source app detection | Windows: `GetForegroundWindow()` + `GetWindowText()` before hotkey fires. Linux: `xdotool getactivewindow getwindowname` (best-effort; empty string if absent). |
| Subject line | **Embedded** as first canvas line. Hover copy (plain text, markdown stripped) in both edit + preview mode. |
| Hotkey conflict | **Warn dialog** before overwriting in-progress session. |
| Re-pyramidize | **Confirm dialog** if canvasText is non-empty. On confirm: auto-trace entry "Canvas (saved)" then run from `originalText`. |
| Original tab | **Always editable** — user can refine original at any time. Trace "Original" entry is always revertable. |
| Error UI | **Inline below step indicator** + Retry (uses current UI state at click time) + Settings shortcut |
| Cancel | **Generic `CancelOperation()` RPC** — one method cancels whichever of Pyramidize/RefineGlobal/Splice is in flight. Silent (no error UI on cancel). |
| "Send back" | **Dedicated `SendBack(text)` RPC** — writes clipboard + restores source-app focus + simulates Ctrl+V. Hidden (not disabled) in manual-paste flow. |
| Rich Text copy | **Client-side** `marked.js` → `ClipboardItem(text/html + text/plain)`. Fallback to Go-side at implementation if WebKit2GTK blocks `navigator.clipboard.write`. |
| Preset saves | **Doc type only** (style + relationship always default per session) |
| State lifetime | **Preserved** across navigation; cleared on app window close |
| Settings tabs | **`[General] [AI Providers] [App Defaults] [About]`** |
| Detection indicator | **Small badge** `● EMAIL` below doc type selector (no confidence %). Disappears on manual type selection. Selector stays on "AUTO" after detection. |
| Empty instruction bar | **Apply / Ctrl+Enter disabled** when instruction is empty. |
| Banner dismiss | **Session-only** — reappears on next visit until a key is set. |

---

### R.5 Out of Scope (this implementation)

| Item | Rationale |
|------|-----------|
| PowerPoint `.pptx` file export | Text output only |
| Version history branch tree in UI | Trace log covers this; visual tree is deferred |
| Cross-user learning | Privacy + infra |
| Streaming output | Single-result model for now |
| AWS Bedrock | Already deferred in codebase |
| Thread/conversation context | Each call is stateless |
| LangChain / orchestration library | Go-native HTTP only |
| Input validation warnings (short/long/structured) | User is trusted; they clicked Pyramidize deliberately |
| Selection-level copy in preview mode beyond hover icon | Hover icon is sufficient |
| Style + relationship saved in presets | Doc type only for presets |
| Per-feature provider selection | Global provider setting; per-feature is future work |

---

### R.6 SOTA Prompting Research

#### Key Findings (March 2026)

**1. Single comprehensive prompt > chained specialist calls**
Modern frontier models (gpt-4o, claude-3-5-sonnet+) handle multi-criteria evaluation and generation in a single well-structured prompt without quality loss. The v1 architecture (6+ sequential LangChain calls) was designed for GPT-3.5-era models. One call with embedded specialist criteria equals or beats 4+ specialist calls at 1/6th the latency and cost.

**2. XML-structured prompting for Claude**
Claude benefits from `<role>`, `<task>`, `<constraints>`, `<examples>`, `<output_format>` XML tags for complex multi-criteria instructions. Reduces instruction confusion when rules overlap (MECE + language preservation + style + tone).

**3. Few-shot examples are critical for format adherence**
For MECE header generation and subject line formatting, 2–3 concrete examples (DE and EN) dramatically improve adherence vs. rules-only prompts. All v1 prompts already had strong examples — carry these forward.

**4. Self-evaluation in the same generation call**
Modern models reliably self-evaluate their output in the same pass. Including `qualityScore` and `qualityFlags[]` in the JSON schema replaces a separate QA call for ~85% of inputs.

**5. Iterative refinement only when triggered**
Replace always-on 4-specialist pipeline with a conditional second call triggered by specific self-reported flags (`MECE_VIOLATION`, `INFO_LOSS`, etc.). Adaptive quality without fixed overhead. **The threshold (default 0.65) is a settings field in App Defaults tab** — not hardcoded — so it can be tuned post-launch without a code change.

**6. One XML-structured prompt for all providers**
`<role>`, `<task>`, `<constraints>`, `<examples>`, `<output_format>` XML tags benefit Claude (attention mechanism). OpenAI and Ollama treat them as plain text and parse correctly regardless. One prompt file per doc type to maintain — no per-provider variants.

**7. Context-aware splice for selection edits**
For section-level AI edits, sending the full canvas + original + selected section gives the model enough context to produce a coherent replacement without rewriting everything. The `rewrittenSection` is spliced back at selection bounds. This may need iteration testing — if context reduction causes coherence issues, escalate to full canvas rewrite.

#### Selected Architecture

```
User selects type manually?
        │ YES                    │ NO
        │                 ┌──── Call 1: Detect
        ▼                 ▼     → {type, lang, conf}
  ┌─────────────────────────────────────────────┐
  │  Call 2: Foundation + Embedded Self-QA       │
  │  (doc-type prompt + MECE + style + examples) │
  │  → {subject, headers[], fullDocument,        │
  │     qualityScore, qualityFlags[]}            │
  └──────────────────┬──────────────────────────┘
                     │
             qualityScore < 0.65?
              YES ▼        NO ▼
      ┌──────────────────┐  Final result
      │  Call 3: Refine  │
      │  (flags as input)│
      └────────┬─────────┘
               ▼
          Final result (with warning if still below threshold)

Canvas AI instructions (independent pipeline):
  Mode A: {fullCanvas, originalText, instruction} → new canvas
  Mode B: {fullCanvas, originalText, selectedText, instruction} → rewrittenSection (splice)
```

---

## Phase I: Implementation Plan

### I.1 New File Layout

```
internal/features/pyramidize/
  service.go           — RPC service: Detect, Pyramidize, RefineGlobal, Splice, AppDefaultsMethods
  types.go             — all Go structs (request/response contracts)
  prompts.go           — all prompt template constants
  api_openai.go        — callOpenAI(client, systemPrompt, userMsg, key) string
  api_claude.go        — callClaude(...)
  api_ollama.go        — callOllama(...)
  json_parse.go        — stripFences, unmarshalRobust
  source_app_map.go    — built-in sourceApp → docType mapping
  service_test.go      — table-driven unit tests

internal/features/settings/
  service.go           — add AppPresets []AppPreset field to Config struct

internal/app/
  wire.go              — add PyramidizeService provider
  wire_gen.go          — regenerated

main.go                — register PyramidizeService

frontend/src/app/
  features/text-enhancement/
    text-enhancement.component.ts    — major rework (becomes Pyramidize page)
    text-enhancement.component.html  — major rework
    text-enhancement.component.scss  — major additions
    text-enhancement.service.ts      — add pyramidize, refineGlobal, splice RPC methods
    text-enhancement.component.spec.ts
    text-enhancement.service.spec.ts
  features/settings/
    settings.component.ts/html       — add App Defaults tab
    settings.component.spec.ts       — update specs
  shared/hover-copy/                 — reusable hover-copy directive/component (canvas + preview)
```

### I.2 Go RPC Types

```go
// ── Pyramidize pipeline ──────────────────────────────────────────────

type PyramidizeRequest struct {
    Text               string `json:"text"`
    DocumentType       string `json:"documentType"`       // "auto"|"email"|"wiki"|"powerpoint"|"memo"
    CommunicationStyle string `json:"communicationStyle"` // "professional"|"casual"|"concise"|"detailed"|"persuasive"|"neutral"|"diplomatic"|"direct"
    RelationshipLevel  string `json:"relationshipLevel"`  // "close"|"professional"|"authority"|"public"
    CustomInstructions string `json:"customInstructions"` // optional, not persisted
}

type PyramidizeResult struct {
    DocumentType      string   `json:"documentType"`
    Language          string   `json:"language"`
    FullDocument      string   `json:"fullDocument"`   // first line = subject/title
    Headers           []string `json:"headers"`
    QualityScore      float64  `json:"qualityScore"`
    QualityFlags      []string `json:"qualityFlags"`
    AppliedRefinement bool     `json:"appliedRefinement"`
    RefinementWarning string   `json:"refinementWarning"` // non-empty if still below threshold after retry
    DetectedType      string   `json:"detectedType"`       // only set when AUTO was used
    DetectedLang      string   `json:"detectedLang"`
    DetectedConfidence float64 `json:"detectedConfidence"`
}

// ── Canvas AI instructions ───────────────────────────────────────────

type RefineGlobalRequest struct {
    FullCanvas         string `json:"fullCanvas"`
    OriginalText       string `json:"originalText"`
    Instruction        string `json:"instruction"`
    DocumentType       string `json:"documentType"`
    CommunicationStyle string `json:"communicationStyle"`
    RelationshipLevel  string `json:"relationshipLevel"`
}

type RefineGlobalResult struct {
    NewCanvas string `json:"newCanvas"`
}

type SpliceRequest struct {
    FullCanvas   string `json:"fullCanvas"`
    OriginalText string `json:"originalText"`
    SelectedText string `json:"selectedText"`
    Instruction  string `json:"instruction"`
}

type SpliceResult struct {
    RewrittenSection string `json:"rewrittenSection"`
}

// ── Cancel & Send Back ───────────────────────────────────────────────

// CancelOperation cancels whichever of Pyramidize/RefineGlobal/Splice is currently in flight.
// Safe to call when no operation is active (no-op).
// RPC method: CancelOperation()

// SendBack writes the result to clipboard, restores focus to the captured source-app window,
// and simulates Ctrl+V to paste. Platform-specific:
//   Windows: SetForegroundWindow(capturedHWND) + SendInput Ctrl+V
//   Linux: xdotool windowfocus <id> && xdotool key ctrl+v (best-effort)
// The source app window handle is captured when the hotkey fires.
// RPC method: SendBack(text string) error

// ── App Presets ──────────────────────────────────────────────────────

type AppPreset struct {
    SourceApp    string `json:"sourceApp"`
    DocumentType string `json:"documentType"`
}

// ── Settings additions ───────────────────────────────────────────────

// Add to Settings struct in internal/features/settings/model.go:
//   AppPresets              []AppPreset `json:"app_presets"`
//   PyramidizeQualityThreshold float64  `json:"pyramidize_quality_threshold"` // default 0.65

// RPC methods: GetAppPresets() []AppPreset, SetAppPreset(preset AppPreset), DeleteAppPreset(sourceApp string)
// GetQualityThreshold() float64, SetQualityThreshold(v float64) error
```

### I.3 Prompt Template Structure

Each doc-type prompt follows this structure (see v1 archive for full text; polish for v2):

```
[ROLE — XML tag for Claude]
You are an expert communication specialist applying the Pyramid Principle.

[LANGUAGE RULE]
Detect and preserve the original language. Never translate.
Code-switching must be preserved.

[DOC-TYPE RULES]
{email | wiki | memo | powerpoint specific rules}
Subject/title format, header rules, MECE principle, priority rule.

[STYLE INJECTION]
Communication style: {style}
Relationship level: {relationship}
{customInstructions if non-empty}

[EXAMPLES — 2-3 concrete, DE + EN]
{v1 examples carried forward, refined}

[SELF-EVALUATION — embedded specialist criteria]
Evaluate as:
- Subject Specialist: format compliance + info density → confidence 0–1
- MECE Specialist: mutual exclusion + collective exhaustion → confidence 0–1
- Completeness Specialist: all people/deadlines/actions preserved → riskScore 0–1 (0=none lost)
- Style Specialist: language consistency + appropriate tone → confidence 0–1
qualityScore = avg(confidences) - (completenessRisk * 0.2)
qualityFlags: list any of ["MECE_VIOLATION", "INFO_LOSS", "STYLE_MISMATCH", "SUBJECT_FORMAT"]

[OUTPUT — JSON only, no fences]
{
  "fullDocument": "complete markdown with subject/title as first line",
  "headers": ["H1", "H2"],
  "language": "de|en|...",
  "qualityScore": 0.0-1.0,
  "qualityFlags": []
}
```

Separate prompt files:
- `prompts_detect.go` — minimal detection prompt; strict enum EMAIL|WIKI|MEMO|POWERPOINT; Go validates + defaults to EMAIL if unrecognised
- `prompts_selfqa.go` — shared `selfQABlock` const (4 specialists, qualityScore formula, flag enum); appended to all 4 doc-type prompts
- `prompts_email.go` — email rules (subject format, MECE headers, language, DE+EN examples) + selfQABlock
- `prompts_wiki.go` — wiki rules (H1 title + H2 sections, content headers) + selfQABlock
- `prompts_memo.go` — memo rules (**TO**/**FROM**/**RE** header block + pyramid body) + selfQABlock
- `prompts_powerpoint.go` — PPT rules (`# Slide N: [Main Message]` + 3–5 bullets per slide) + selfQABlock
- `prompts_refine.go` — `buildRefinePrompt(original, failedOutput string, flags []string) string`; human-readable remediation instructions per flag (switch/case); `originalText` is hard constraint here (restoring completeness is the goal of refinement)
- `prompts_canvas.go` — global canvas instruction (`originalText` is **soft** context only, user instruction takes priority) + selection splice prompt

### I.4 Angular State Model (module-level)

```typescript
// Module-level — survives navigation and hide/show cycles until window close
let originalText    = '';
let pyramidizedText = '';    // snapshot after most recent foundation call
let canvasText      = '';    // live working surface
let sourceApp       = '';    // captured from hotkey; empty in manual flow
let docType         = 'auto';
let commStyle       = 'professional';
let relLevel        = 'professional';

interface TraceEntry {
  id:        string;
  label:     string;          // "Original" | "Pyramidize" | "AI: ..." | "Checkpoint" | "Reverted to: ..."
  snapshot:  string;          // canvas text at this point
  timestamp: Date;
}
let traceLog: TraceEntry[]  = [];
let activeTab: 'original' | 'canvas' = 'original';
let isPreviewMode            = false;   // canvas tab: rendered vs raw textarea
let traceLogOpen             = false;
let wasCancelled             = false;   // set true before CancelOperation() RPC; cleared on next operation start
```

### I.5 Angular UI Component Structure

**Left settings panel:**
- Doc type selector (PrimeNG Select): AUTO, EMAIL, WIKI, POWERPOINT, MEMO
  - After AUTO detection: small badge `● EMAIL` appears below selector (no confidence %). Disappears on manual selection. Selector remains on "AUTO".
  - Eyecatcher overlay when new source app detected (animated border + label + "Set as default" CTA)
- Communication style selector
- Relationship level selector
- Custom instructions textarea (placeholder: "Optional: 'Use formal Sie', 'Mention Q1 deadline'…")
- [Pyramidize] button with `Ctrl+↵` annotation
- Provider badge (muted): `⚫ [model] · [provider]` → click → navigate to Settings

**Canvas area:**
- Tab bar: [Original] [Canvas] — Original tab is always editable
- Canvas tab: mode toggle [Edit ●] [Preview]
- Edit mode: `<textarea>` with hover-copy overlay (mouse position tracking, copies plain text)
- Preview mode: rendered HTML via `marked.js` pipe (`[innerHTML]="canvasText | markdown"`) with per-paragraph copy buttons (plain text). No selection bubble in preview.
- Step indicator / progress / error replaces/extends the canvas header area; Cancel button always visible during any in-progress operation
- Fixed instruction bar at bottom: `[Ask AI about whole document… [Apply Ctrl+↵]]` — Apply and Ctrl+Enter disabled when empty
- Selection-aware floating bubble (appears when selection is non-empty in **textarea only** — edit mode)
- Action row below instruction bar: [Copy as Markdown] [Copy as Rich Text] [Send back to App] (last one hidden if no sourceApp)

**Right trace log panel:**
- Panel header: `Trace Log [+]` (+ = save checkpoint)
- Entry list: each entry shows label + time; active entry has indicator
- Peek mode: clicking entry expands inline read-only preview
- "Revert to here" button per entry
- Collapsed by default (icon-only strip)

### I.6 Settings: App Defaults Tab

New tab: `[General] [AI Providers] [App Defaults] [About]`

```
[General]  [AI Providers]  [App Defaults]  [About]

App Defaults
─────────────────────────────────────────────────────
Pyramidize Quality Threshold
[0.65    ]
Values below this score trigger a refinement call.
(0–1, default 0.65)

─────────────────────────────────────────────────────
App Presets

No app defaults saved yet. Use Pyramidize
with the global hotkey to detect apps automatically.

─────────────────────────────────────────────────────
│ Microsoft Outlook    │ EMAIL       │ ✏️ 🗑️ │
│ VS Code              │ MEMO        │ ✏️ 🗑️ │
│ Notepad++            │ MEMO        │ ✏️ 🗑️ │
─────────────────────────────────────────────────────
[+ Add manually]
```

### I.7 Manual E2E Verification Checklist

**Scenario A — First-time user, no API key:**
1. Open app → Pyramidize view → amber banner visible
2. [Open Settings] navigates to providers → configure key → return
3. Banner gone, Pyramidize button enabled ✓

**Scenario B — Hotkey from Outlook, new app:**
1. Select email text in Outlook → hotkey → Pyramidize opens, text pre-filled
2. Eyecatcher on doc type selector: "Detected: Outlook → Email" ✓
3. Click "Set as default for Outlook" → eyecatcher fades ✓
4. Close app → repeat hotkey → doc type pre-selected silently, no eyecatcher ✓

**Scenario C — Core pyramidize flow (email, German):**
1. Paste unstructured German status email → leave AUTO → click Pyramidize
2. Step indicator: "Step 1/2: Detecting…" → "Step 2/2: Structuring…" ✓
3. Canvas tab auto-activates → textarea shows formatted markdown ✓
4. Subject line format: `[Main] | [Details] | [Actions]` ✓
5. MECE headers: each header is a standalone content statement ✓
6. Trace log has entries: "Original" + "Pyramidize" ✓

**Scenario D — Canvas editing:**
1. From Scenario C: select header text in textarea → "✨ Ask AI" bubble appears ✓
2. Type instruction → Enter → canvas updated, trace entry added ✓
3. Type in instruction bar → Ctrl+Enter → whole canvas updated, trace entry added ✓
4. Click [+] in trace log → checkpoint entry added ✓
5. Click "Pyramidize (12:02)" → peek mode opens ✓
6. Click "Revert to here" → canvas restored + new "Reverted to…" trace entry ✓

**Scenario E — Copy and send back (hotkey flow):**
1. [Copy as Markdown] → paste into text editor, verify markdown ✓
2. [Copy as Rich Text] → paste into email client, verify formatting preserved ✓
3. [Send back to Outlook] → text sent to Outlook compose window ✓

**Scenario F — Cancel during operation:**
1. Click Pyramidize → immediately click Cancel → operation aborted, original preserved ✓

**Scenario G — Error recovery:**
1. Disconnect network → click Pyramidize → timeout → inline error shown ✓
2. Click [Retry] → retries from failed step ✓
3. Canvas state unchanged (original text still there) ✓

**Scenario H — Wiki flow (English):**
1. Paste technical documentation → select WIKI → Pyramidize
2. Output: H1 title + H2 sections, no subject line ✓
3. Preview mode: renders with markdown headers and bullets ✓

### I.8 Implementation Execution Order

1. **Go backend first** (no UI needed to test):
   - `types.go` → `json_parse.go` → `source_app_map.go`
   - `prompts_selfqa.go` (shared self-QA block first, then doc-type prompts use it)
   - `prompts_detect.go` + `prompts_email.go` (start with email)
   - `service.go`: Detect + Pyramidize methods
   - `service_test.go`: table-driven tests with mock HTTP
   - Settings: add AppPresets + PyramidizeQualityThreshold to Config, implement Get/Set/Delete/GetThreshold/SetThreshold
   - Remaining prompts (wiki, memo, ppt, refine, canvas)
   - RefineGlobal + Splice methods + tests

2. **Wire DI + bindings**:
   - `wire gen ./internal/app/` → `wails3 generate bindings`

3. **Angular — Settings tab first** (simpler, validates AppPresets RPC):
   - App Defaults tab (list, edit, delete)

4. **Angular — Pyramidize page**:
   - State model + module-level vars
   - Left panel + basic Pyramidize call
   - Canvas textarea + preview toggle
   - Hover copy (preview mode first, then textarea overlay)
   - Trace log panel
   - Global instruction bar
   - Selection bubble
   - Keyboard shortcuts
   - Error/progress states
   - Empty state + paste button
   - Provider badge + API key warning
   - Send back / copy actions
   - Eyecatcher for new source apps

5. **Vitest specs** (component + service)

6. **Manual E2E** per §I.7 checklist

---

*Requirements finalized: 2026-03-08 — All gaps resolved 2026-03-08 — Ready to begin ENG-01*
