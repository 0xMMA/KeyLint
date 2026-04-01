# Logo Unfolding Animation — Requirements

## Overview

When the sidebar expands (either via the collapse-toggle button or hover), the compact
"KL" logo must **visually unfold** into "KeyLint" — not by swapping elements or fading,
but by growing the hidden characters in-place so the existing letters appear to push apart
and reveal what was always between and after them.

## Behaviour

| State | Visible text | What is animating |
|-------|-------------|-------------------|
| Collapsed | **K** **L** | — |
| Expanding | **K** *ey* **L** *int* | "ey" grows left→right between K and L; "int" grows left→right after L |
| Expanded | **Key** **Lint** | — |

- "K" never moves; it is always anchored at the left of the logo area.
- "L" is **pushed right** as "ey" grows between it and "K".
- "int" grows after "L" simultaneously.
- The reverse (collapsing) plays the animation backwards: "ey" and "int" contract,
  pulling "L" back toward "K".

## Colour

- "K" and "ey" → near-white (`--p-surface-50`)
- "L" and "int" → orange (`--p-primary-color`)

(Same colour split as before — "Key" is white, "Lint" is orange.)

## Timing

- Animation duration: **0.18 s ease** — matches the sidebar width transition (0.20 s).
- No delay: text starts growing the moment the sidebar starts expanding, so both
  finish at roughly the same time.
- Collapsing: same duration, no delay, so "KL" is back before the sidebar fully closes.

## Technical approach

Use individual `<span>` elements for each character group, with CSS `max-width` and
`overflow: hidden` on the "expand" spans (`.logo-reveal`):

```html
<div class="layout-logo">
  <span class="logo-k logo-key">K</span>
  <span class="logo-reveal logo-ey logo-key">ey</span>
  <span class="logo-l logo-lint">L</span>
  <span class="logo-reveal logo-int logo-lint">int</span>
</div>
```

- `.logo-reveal` → `overflow: hidden; max-width: 4em; white-space: nowrap;`
- Collapsed/not-hover-expanded → `max-width: 0; transition: max-width 0.18s ease;`
- Expanded / hover-expanded → `max-width: 4em; transition: max-width 0.18s ease;`
- No `@if/@else` in the template — both states live in CSS only.
- `.layout-logo` keeps `display: flex; align-items: center; justify-content: center`.
  Flex layout means vertical alignment is handled by the container, avoiding the
  `inline-block` baseline shift that causes misalignment with `overflow: hidden`.

## Test coverage required

Each test must pass reliably (using `waitForTimeout(350)` after state change):

1. **Collapsed state**: `.logo-ey` and `.logo-int` have rendered width ≈ 0.
2. **Expanded state**: `.logo-ey` and `.logo-int` have rendered width > 5 px.
3. **Hover-expanded state**: same as expanded.
4. **"K" is white** (check `.logo-k` computed color).
5. **"L" is orange** (check `.logo-l` computed color).
6. **"ey" colour matches "K" colour** (both white/near-white).
7. **"int" colour matches "L" colour** (both orange).
8. **"KL" is horizontally centered** in the collapsed sidebar.
9. **No layout shift** in `.layout-main` when sidebar hover-expands (regression).

## Out of scope

- Easing curves beyond `ease` (cubic-bezier fine-tuning can come later).
- Per-character stagger (all chars in a group expand together).
- RTL support.
