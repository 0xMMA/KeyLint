# KeyLint — Feature Parity TODO

Remaining gaps between v1 (Rust/Tauri) and v2 (Go/Wails).
For Pyramidize-specific status, see `docs/pyramidize/`.

---

## Silent Fix

- [x] **Auto-paste to source app** — `PasteToForeground` on both platforms
- [ ] **Linux hotkey** — currently a no-op stub (`service_linux.go`). Wire up a real global
      shortcut (e.g. `github.com/robotn/gohook` or `xbindkeys` integration).
- [ ] **HTML clipboard support** — detect foreground app (Outlook, Word, LibreOffice),
      convert Markdown output to HTML, write both CF_HTML and CF_TEXT to clipboard.

---

## Pyramidize

- [x] Full pipeline (detect -> foundation -> self-QA -> refine)
- [x] CLI mode (`-fix`, `-pyramidize`)
- [x] Evaluation framework (deterministic + LLM-as-judge)
- [x] All UI controls (doc type, style, relationship, custom instructions, canvas, trace log)
- [ ] **HTML clipboard paste-back** — convert Markdown to HTML for Outlook/Teams
- [ ] **Parallel specialist agents** — v1 had 4 independent specialists; currently simplified as self-eval. See `docs/pyramidize/adr-001-pipeline-architecture.md`.

---

## Platform

- [x] Minimize to tray on close
- [x] Tray icon click brings window to front
- [x] Version + update indicator in nav
