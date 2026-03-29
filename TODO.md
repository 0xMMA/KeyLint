# KeyLint — Feature Parity TODO

Audit of gaps between v1 (Rust/Tauri) and v2 (Go/Wails).
Focus: the two core features — **Silent Fix** and **Pyramidize**.

---

## System Tray & Window Lifecycle

- [x] **Minimize to tray on close** — `ApplicationShouldTerminateAfterLastWindowClosed: false` set in
      `main.go`; window-close event calls `window.Hide()`.

- [x] **Tray icon click / double-click brings window to front** — `tray.OnClick` and
      `tray.OnDoubleClick` handlers added in `internal/features/tray/service.go`.

---

## Silent Fix

- [x] **Auto-paste to source app** — `PasteToForeground` implemented on both platforms:
      Windows via Win32 `SendInput` (`paste_windows.go`), Linux via `xdotool` (`paste_linux.go`).

- [ ] **Linux hotkey** — currently a no-op stub (`service_linux.go`). Wire up a real global
      shortcut (e.g. `github.com/robotn/gohook` or `xbindkeys` integration).

- [ ] **HTML clipboard support** — detect foreground app (Outlook, Word, LibreOffice, etc.),
      convert Markdown output to HTML, write both CF_HTML and CF_TEXT to clipboard.
      v1 had `HtmlClipboardService` with app-name regex matching.

---

## Version & Updates

- [x] **Version + update indicator in main nav** *(v4.0.0-alpha finding)* — display the app version
      in small text at the bottom-left of the shell nav alongside a single icon that lights up when
      an update is available. Clicking the icon (or version text) should navigate to Settings → About.
      The version string is already available via `wails.getVersion()`; update status via
      `wails.checkForUpdate()`. Currently only visible in Settings → About.

---

## Pyramidize (Advanced Mode)

### Pipeline (Generate → Self-QA → Refine)

- [x] **Document type detection** — LLM classifies input as EMAIL / WIKI / POWERPOINT / MEMO
      (or user selects manually). Returns `{type, language, confidence}`.
      See `internal/features/pyramidize/prompts_detect.go`, `service.go:detect()`.

- [x] **Oneshot foundation generator** — document-type-specific prompt templates (German + English)
      that convert raw text into a structured document with embedded self-QA.
      Output: `{fullDocument, headers[], language, qualityScore, qualityFlags[]}`.
      See `prompts_email.go`, `prompts_wiki.go`, `prompts_memo.go`, `prompts_powerpoint.go`.

- [x] **Self-evaluation specialists** — 5 specialist lenses embedded in the foundation call
      (subject/title, MECE, completeness, style, fidelity). Returns qualityScore + qualityFlags.
      Design decision: single-call self-eval instead of parallel specialist agents for efficiency.
      See `prompts_selfqa.go`.

- [x] **Quality threshold + conditional refinement** — if qualityScore < threshold (default 0.65),
      a targeted refinement pass runs using the qualityFlags to fix specific issues.
      Threshold configurable via `SetQualityThreshold()`. See `service.go`, `prompts_refine.go`.

- [ ] **Parallel specialist agents (v1 design, not yet ported)** — run separate specialist LLM
      calls concurrently, each returning individual confidence scores. Integration coordinator
      selectively applies improvements where confidence > 0.7. Currently simplified as self-eval.

### Evaluation Pipeline (CLI + automated tests)

- [x] **CLI mode** — `-fix` and `-pyramidize` commands dispatched headlessly from `main.go`.
      See `internal/cli/`.

- [x] **Deterministic eval checks** — structure, info coverage, hallucination detection.
      See `eval_deterministic.go`.

- [x] **LLM-as-judge scoring** — 5-dimension quality assessment against baseline test data.
      See `eval_judge.go`, `eval_test.go`.

- [x] **Eval run logging** — results persisted to `test-data/eval-runs/<timestamp>/`.
      Run with: `EVAL_PROVIDER=claude go test -tags eval ./internal/features/pyramidize/ -v -timeout 600s`

### UI Controls

- [x] Document type selector (AUTO / EMAIL / WIKI / POWERPOINT / MEMO)
- [x] Communication style selector
- [x] Relationship level selector
- [x] Custom instructions textarea
- [x] Markdown rendering for output (custom Angular pipe)
- [x] Editable output (canvas textarea with edit/preview toggle)
- [x] Tab view: Original vs Editor

### Clipboard integration

- [ ] **HTML clipboard paste-back** — same as Silent Fix: convert Markdown output to HTML
      and paste to source app with proper MIME types.

---

## Priority Order

1. ~~**Auto-paste to source app**~~ ✓ done
2. ~~**Minimize to tray on close**~~ ✓ done
3. ~~**Tray icon click brings window to front**~~ ✓ done
4. ~~**Version + update indicator in nav**~~ ✓ done
5. Pyramidize pipeline (core value proposition)
6. Pyramidize UI controls
7. Linux hotkey
8. HTML clipboard support
