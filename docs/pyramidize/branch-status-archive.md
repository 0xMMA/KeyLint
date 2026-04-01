# Branch Status — feat/pyramidize

> Last updated: 2026-03-14

## What this branch contains

This branch implements the full **Pyramidize** feature (AI-powered document
structuring) plus a thorough UX / sidebar polish pass. It is ready for review
against `main`.

---

## Commits on this branch (above main)

| SHA | Subject |
|-----|---------|
| `708221f` | feat(shell,pyramidize): UX pass — sidebar polish, model upgrades, clipboard resilience |
| `4defafa` | fix(dev): start ng serve before Go build to avoid cold-start race condition |
| `b4b61c1` | feat(pyramidize): implement full Pyramidize feature (ENG-01 through ENG-12) |

---

## Feature: Pyramidize (`/enhance` route)

Full spec: `PYRAMIDIZE.md` / `PYRAMIDIZE-UX.md`

### What it does
- Paste or type any text → AI restructures it as a well-structured document
  (Email / Wiki / PowerPoint / Memo, auto-detected or manually chosen)
- 2-call adaptive pipeline: detect → structure → optional self-QA refine if
  quality score < configurable threshold
- Rich editing canvas with Edit / Preview toggle
- Trace log — every operation is snapshotted; hover to preview, click to pin,
  revert to any point
- Selection bubble (EDIT mode) — select text, give an AI instruction, apply
- Global instruction bar — rewrite whole canvas with one instruction
- Copy as Markdown or Rich Text (HTML+plain clipboard item)
- Send back to source app (via `xdotool` on Linux)
- Source app detection badge
- Cancel any in-flight operation

### New Go package
`internal/features/pyramidize/` — service, three provider adapters
(claude / openai / ollama), prompt files, self-QA, JSON parser, capture

### New RPCs (registered in `main.go`)
`Pyramidize`, `RefineGlobal`, `Splice`, `CancelOperation`, `SendBack`,
`GetSourceApp`, `GetQualityThreshold`, `SetQualityThreshold`

---

## UX / Sidebar polish (applied on top of Pyramidize)

### Shell sidebar fixes (all covered by Playwright tests)

| Fix | File(s) |
|-----|---------|
| Nav icons not centered in collapsed sidebar | `shell.component.scss` |
| SVG pyramid icon stays grey on hover (should turn orange) | `shell.component.scss` |
| Version-row 16px dead click-strip when collapsed | `shell.component.scss` |
| Scrollbar flicker on hover-expand | `shell.component.scss` (absolute sidebar overlay) |
| Active Pyramidize icon orange-on-orange (invisible) | `shell.component.scss` |
| Collapsed logo showed only "K" (should be "KL") | `shell.component.ts` |
| Collapse button too small (< 40px) | `shell.component.scss` |
| Sidebar didn't hover-expand as overlay | `shell.component.scss` / `.ts` |
| **KL → KeyLint logo transition artifacts** | `shell.component.ts/scss` — CSS `max-width` unfold animation (see `LOGO-ANIMATION.md`) |
| **Active nav item drops 1-2px on hover-expand** | `shell.component.scss` — `span { line-height: 1 }` fix |

### Pyramidize component fixes

| Fix | Detail |
|-----|--------|
| "Canvas" tab renamed to "Editor" | All user-visible labels updated |
| Quality threshold input white background in dark mode | Replaced `<input type="number">` with `<p-inputnumber>` |
| Trace entries show preview on hover (non-sticky) | `hoverEntry` + `activeEntry` getter; click makes sticky |

---

## Logo animation (`LOGO-ANIMATION.md`)

The collapsed sidebar shows **KL**. When expanding (button click or hover),
the hidden characters unfold in-place:

```
K [ey grows→] L [int grows→]   →   KeyLint
```

Implementation: four inline `<span>` elements; `.logo-reveal` spans have
`overflow: hidden; max-width: 4em; transition: max-width 0.18s ease`.
Collapsed state sets `max-width: 0` via `.layout-sidebar.collapsed:not(.hover-expanded)`.
No JavaScript, no `@if/@else` — pure CSS.

---

## Test coverage

| File | Tests | Scope |
|------|-------|-------|
| `e2e/shell-menu.spec.ts` | 24 | Dimensions, navigation, collapse basics |
| `e2e/shell-menu-deep.spec.ts` | 19 | Logo, icon consistency, version row, footer |
| `e2e/shell-menu-deep2.spec.ts` | 12 | Icon width parity, hover colours, scroll |
| `e2e/shell-menu-deep3.spec.ts` | 10 | Version-row dead space, layout-main dims |
| `e2e/shell-menu-deep4.spec.ts` | 19 | All 5 session-2 fixes + logo animation |
| `e2e/shell-menu-deep5.spec.ts` | 7 | Nav-item vertical stability on hover-expand |
| `src/**/*.spec.ts` | 108 | Vitest unit tests (all passing) |

E2E tests require `ng serve` on `:4200`. Run with `npx playwright test`.
Unit tests: `cd frontend && npm test`.

---

## Known limitations / not yet done

- Bedrock provider adapter (stubs exist, not wired)
- App Presets UI in Settings (`GetAppPresets` / `SetAppPreset` RPCs exist)
- Windows `SendInput` for Send Back (Linux xdotool works)
- E2E tests for Pyramidize canvas interactions (covered by unit tests only)
- **Bug:** collapsed sidebar version row renders double `vv` prefix — needs investigation and fix
