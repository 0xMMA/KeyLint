# PYRAMIDIZE-UX — UX Polish & Model Upgrades

Companion plan to PYRAMIDIZE.md. Covers post-ENG-12 UX improvements and model-selection work.

---

## Model Strategy (all providers)

### Anthropic
Official aliases (documented, forward-compatible — no snapshot date = always latest snapshot of that generation):
- `claude-sonnet-4-6` → Claude Sonnet 4.6 (best balance; **default for Pyramidize**)
- `claude-opus-4-6`   → Claude Opus 4.6 (highest quality, higher cost)
- `claude-haiku-4-5`  → Claude Haiku 4.5 (fastest, lowest cost)

**Change from:** `claude-haiku-4-5-20251001` → **`claude-sonnet-4-6`** (hardcoded default)

### OpenAI
Rolling aliases (no date suffix required). Current generation is **GPT-4.1** (Apr 2025);
GPT-4o is the previous generation. GPT-5 exists as of Aug 2025.

- `gpt-4.1-nano` → GPT-4.1 Nano — fastest/cheapest
- `gpt-4.1-mini` → GPT-4.1 Mini — fast/cheap
- `gpt-4.1`      → GPT-4.1 — balanced
- `gpt-5.1-mini` → GPT-5.1 Mini — fast, capable
- `gpt-5.2`      → GPT-5.2 — **default for Pyramidize** (Dec 2025, pinned: `gpt-5.2-2025-12-11`)
- `gpt-5.2-pro`  → GPT-5.2 Pro — most capable
- `o3`           → o3 — reasoning tasks

`gpt-4o` / `gpt-4o-mini` are old generation — do not use as defaults.

**Change from:** `gpt-4o-mini` → **`gpt-5.2`**

### Ollama
Docker-style tags — omitting tag defaults to `:latest`. Models to surface in selector:
- `llama3.2` (default)
- `mistral`
- `gemma3`
- `phi4`
- `qwen2.5`

User can also type a custom model name directly.

---

## UX-01 — Inline Provider + Model Selector (Pyramidize)

**Goal:** Replace the "Change Provider" button in the error row with a persistent, compact provider+model selector visible in the left settings panel (below the Document Type selector).

**Location:** `text-enhancement.component.ts` — left panel, between doc-type selector and comm-style selector.

**Design:**
```
Provider   [Anthropic ▼]
Model      [claude-sonnet-4-6 ▼]
```
- Two stacked `<p-select>` dropdowns (small size)
- Provider options: Anthropic | OpenAI | Ollama
- Model list changes based on provider (static map in component)
- Selection persists as module-level `let` vars (`selectedProvider`, `selectedModel`)
- These are passed into the `PyramidizeRequest` (add `provider` + `model` fields to the RPC)
- Remove "Change Provider" button from error row entirely; keep only "Retry"
- **Fix page** gets the same selector (UX-01b below)

**Go changes:**
- `types.go`: add `Provider string` and `Model string` to `PyramidizeRequest`, `RefineGlobalRequest`, `SpliceRequest`
- `service.go`: pass `req.Provider`/`req.Model` to the api_*.go call instead of reading from settings; fall back to settings provider if `req.Provider` is empty
- `api_claude.go`: accept model as parameter, default `claude-sonnet-4-6`
- `api_openai.go`: accept model as parameter, default `gpt-5.2`
- `api_ollama.go`: accept model as parameter, default `llama3.2`

**Angular changes:**
- `wails.service.ts`: update `BROWSER_MODE_DEFAULTS` to reflect new fields
- `text-enhancement.component.ts`: add provider/model selectors to left panel; add `selectedProvider`/`selectedModel` module-level vars; update `pyramidizeParams()` builder

---

## UX-01b — Inline Provider + Model Selector (Fix / Quick Fix page)

**Goal:** Add same compact provider+model selector to the Fix page, above the Fix button row.

**Location:** `fix.component.ts` — below the textareas, above the action row.

**Design:** Same two-dropdown layout as UX-01. Module-level vars `_fixProvider`, `_fixModel`.

**Go changes:**
- `enhance/service.go` (`enhance()` method): accept optional provider+model override (or add a new `EnhanceWithOptions` RPC)
- Actually: add `provider` + `model` to the existing Enhance RPC call (or a new `EnhanceOptions` struct)

---

## UX-02 — Collapsible Main Menu

**Goal:** Sidebar can collapse to icon-only strip (~48px), toggled by a chevron button at the bottom of the sidebar.

**Location:** `shell.component.ts` + `shell.component.scss`

**Design:**
- Module-level `let sidebarCollapsed = false` (persists across navigation)
- `.layout-sidebar` gets class `.collapsed` when collapsed
- Collapsed width: `48px`; expanded: `17rem`
- Nav items: hide `<span>` text when collapsed (CSS `display:none` on span in collapsed state)
- Logo: show only key icon when collapsed (hide "Lint" text or show abbreviated "K")
- Chevron toggle button at bottom of sidebar (rotates 180° when collapsed)
- Version text hidden when collapsed
- Smooth CSS transition: `width 0.2s ease`
- `pTooltip` on each nav icon when collapsed (shows label on hover)

**No Go changes.**

---

## UX-03 — Error Message Clipping + Copy

**Goal:** Error messages from the AI (sometimes very long) should be clipped to ~2 lines max with a copy icon. Full text accessible via copy.

**Location:** `text-enhancement.component.ts` error row (lines 283-290)

**Design:**
```
❌ [truncated error text, 2 lines max...]  [⧉ copy]   [Retry]
```
- Error text container: `max-height: 2.8em; overflow: hidden; text-overflow: ellipsis`
  (or use `-webkit-line-clamp: 2`)
- Copy icon button (`pi pi-copy`) next to error text — copies full error to clipboard
- Remove "Change Provider" button (replaced by UX-01 inline selector)
- `p-tooltip` on the error text showing full message on hover (with `appendTo="body"`)

**No Go changes.**

---

## UX-04 — Quality Threshold in Pyramidize UI

**Goal:** Move the quality threshold control from Settings › App Defaults into the Pyramidize left panel (advanced section), so users can tune it in-context.

**Location:** `text-enhancement.component.ts` — bottom of left panel, collapsible "Advanced" section.

**Design:**
```
▸ Advanced
  Quality threshold  [0.65 ▲▼]   (0–1)
```
- PrimeNG `<p-inputnumber>` with step 0.05, min 0, max 1
- Module-level `let qualityThreshold = 0.65` — loaded via `getQualityThreshold()` on init, saved via `setQualityThreshold()` on change (debounced or on blur)
- Collapse/expand section with a small toggle (module-level `let advancedOpen = false`)
- **Remove** quality threshold from Settings › App Defaults tab (or keep as read-only display, or remove entirely — remove is cleaner)

**No Go changes** (RPCs `GetQualityThreshold`/`SetQualityThreshold` already exist).

---

## UX-05 — Checkpoint Tooltip Fix

**Goal:** "Add checkpoint" tooltip wraps per-character because the tooltip container has no min-width and appears inside a constrained flex container.

**Location:** `text-enhancement.component.ts` trace panel header (lines 369-377)

**Fix:**
- Add `appendTo="body"` to all `pTooltip` directives in the trace panel (and ideally globally in the component)
- Add `tooltipPosition="left"` or `"top"` as appropriate
- Add `[tooltipOptions]="{ showDelay: 400 }"` to reduce accidental triggers
- Add `style="white-space: nowrap"` override via `tooltipStyleClass` if needed

The core issue is that without `appendTo="body"`, PrimeNG renders the tooltip inside the nearest `position:relative` ancestor which is the 260px-wide trace panel — causing the tooltip text to wrap.

**No Go changes.**

---

## UX-06 — Trace Log Peek as Central Overlay

**Goal:** When a trace log entry is clicked (peek), instead of showing the preview in a tiny bottom-right sub-panel within the trace panel itself, show it as a full-width overlay/dialog over the central canvas area.

**Location:** `text-enhancement.component.ts` trace panel peek section (lines 410-432)

**Design:**
- Remove the inline `.trace-peek` sub-panel from inside the trace panel
- Add a `<p-dialog>` or absolutely-positioned overlay panel anchored over the `.canvas-area`
- Dialog has:
  - Header: entry label + timestamp + close button
  - Body: `<pre>` with full snapshot text, scrollable, fills available space
  - Footer: "Revert to here" danger button + "Close" secondary button
- The trace panel entry click sets `peekEntry` (existing var) — dialog opens when `peekEntry != null`
- Markdown preview option: if snapshot looks like markdown (has `#` headers), render via `MarkdownPipe`

**Styling:**
- Overlay width: same as `.canvas-area` (use `position: absolute; inset: 0` within `.canvas-area` for a native overlay, or use PrimeNG Dialog with `appendTo=".canvas-area"`)
- Prefer native CSS overlay over PrimeNG Dialog (avoids z-index issues with sidebar)

**No Go changes.**

---

## UX-07 — Canvas Preview Height / Scroll Fix

**Goal:** In preview mode, the markdown preview expands to full content height and pushes the instruction bar and action row off-screen.

**Location:** `text-enhancement.component.ts` canvas area

**Fix:**
- `.canvas-area` is already `flex-direction: column` — but the preview div has `min-height: 320px` with no `max-height` or `flex: 1` + `overflow-y: auto`
- Change `.canvas-preview` to use `flex: 1; overflow-y: auto` (same treatment as `.canvas-textarea`)
- Wrap the tab panels + instruction bar in a flex column: tab panel content gets `flex: 1; overflow: hidden`
- The instruction bar and action row must be `flex-shrink: 0` (already are via fixed height, but verify)
- This ensures the preview never grows past the available viewport height

**Specifically:**
```scss
// In text-enhancement.component.ts styles:
.canvas-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;  // already set
  min-height: 0;     // ADD THIS — critical for nested flexbox shrink
}

.canvas-preview {
  flex: 1;           // CHANGE from min-height: 320px
  overflow-y: auto;  // already set
  min-height: 0;     // ADD for flexbox
}

.canvas-textarea {
  flex: 1;           // ADD (was min-height: 320px + resize:vertical)
  resize: none;      // CHANGE (resize:vertical fights flexbox)
  min-height: 0;
}
```
Also ensure the PrimeNG `<p-tabs>` component itself is `flex: 1; overflow: hidden; display: flex; flex-direction: column` and its tab panel content wrapper likewise.

**No Go changes.**

---

## Implementation Order

| # | Task | Effort | Go? | Angular? |
|---|------|--------|-----|----------|
| UX-07 | Canvas preview overflow fix | XS | — | ✓ |
| UX-05 | Tooltip fix | XS | — | ✓ |
| UX-03 | Error clipping + copy | S | — | ✓ |
| UX-02 | Collapsible sidebar | S | — | ✓ |
| UX-04 | Quality threshold in Pyramidize UI | S | — | ✓ |
| UX-06 | Trace peek overlay | M | — | ✓ |
| UX-01 | Provider+model selector (Pyramidize) | M | ✓ | ✓ |
| UX-01b | Provider+model selector (Fix) | S | ✓ | ✓ |

**Suggested grouping:**
- **Sprint A (pure Angular, no bindings regen):** UX-07, UX-05, UX-03, UX-02, UX-04, UX-06
- **Sprint B (requires Go + bindings regen):** UX-01, UX-01b (model defaults upgrade included)

---

## Files to Change

### Go
- `internal/features/pyramidize/api_claude.go` — model param, default `claude-sonnet-4-6`
- `internal/features/pyramidize/api_openai.go` — model param, default `gpt-4o`
- `internal/features/pyramidize/api_ollama.go` — model param, default `llama3.2`
- `internal/features/pyramidize/types.go` — add `Provider`, `Model` to request types
- `internal/features/pyramidize/service.go` — route to correct api_*.go based on `req.Provider`
- `internal/features/enhance/service.go` — add `EnhanceWithModel(provider, model, text)` RPC

### Angular
- `frontend/src/app/features/text-enhancement/text-enhancement.component.ts` — UX-01, 03, 04, 05, 06, 07
- `frontend/src/app/features/fix/fix.component.ts` — UX-01b
- `frontend/src/app/layout/shell.component.ts` + `.scss` — UX-02
- `frontend/src/app/features/settings/settings.component.ts` — remove quality threshold (UX-04)
- `frontend/src/app/core/wails.service.ts` — new RPC method for `EnhanceWithModel`
- `frontend/src/testing/wails-mock.ts` — mock for new RPC

### Bindings (auto-generated, run after Go changes)
- `frontend/bindings/keylint/internal/features/pyramidize/` — after types.go change
- `frontend/bindings/keylint/internal/features/enhance/` — after enhance/service.go change
