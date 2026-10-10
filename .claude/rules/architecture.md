# Architecture Rules

## Service Registration

After adding or changing a Go service: run `wire gen ./internal/app/` then `wails3 generate bindings` — both required before building.

Registration pattern in `main.go`: `wailsApp.RegisterService(application.NewService(svc))` — always wrap with `application.NewService()`.

## RPC Bridge

All frontend→Go calls go through `WailsService` (`frontend/src/app/core/wails.service.ts`). Components never import from `frontend/bindings/` directly.

## AI API Calls

AI requests must go through the Go backend. Every provider HTTP call lives in `internal/llm` — features build an `llm.Request`, call `Client.Complete`, and never talk to a provider API directly. No `net/http` calls to provider APIs outside that package.

The frontend has no exception to this and no longer has a browser-mode fallback: `TextEnhancementService` passes straight through to `WailsService`, and nothing under `frontend/src` calls a provider. A skipped E2E spec still carries the remains of the old fallback — see `testing.md`.

## Dark Mode

Dark only, for now: `<html class="app-dark">` in `index.html`, and `ShellComponent.applyTheme()` keeps it whatever `theme_preference` says — only the dark theme is styled (#24). The class goes on `<html>`, never `<body>`: PrimeNG's component tokens are `:root` aliases that resolve where declared, so a class below the root leaves them light (#28). The field stays in the settings model for the light theme (#25), which is where reading it comes back. Never use `window.matchMedia` for theme detection — jsdom does not define it at all, so a spec that reaches it throws rather than returning light. (Specs that open a PrimeNG overlay stub it; see `testing.md`.)

## PrimeNG v21

Import individual symbols: `import { Tabs, TabList, Tab } from 'primeng/tabs'` — not module classes.

## Styling: tokens and Tailwind

PrimeNG is being replaced (roadmap E11), so app styles never read PrimeNG's `--p-*` tokens. Colours, surfaces and radii come from the `--kl-*` tokens in `frontend/src/theme.css` (`var(--kl-muted)`, `var(--kl-line)`, …); the only `--p-*` left in app code are the PrimeNG overrides in `styles.scss`, which patch PrimeNG and go with it. New colour needed? Add a token there, don't hard-code it or borrow a `--p-*`.

Layout utilities are Tailwind v4 (`mt-4`, `flex`, `gap-4`, `text-muted`, `bg-sheet`) — PrimeFlex is gone, and Tailwind's spacing step is 0.25rem (`mt-4` = 1rem). Only KeyLint's colours exist as utilities. Preflight is off while PrimeNG is in, so no global reset. Utilities sit in `@layer utilities`, and everything else is unlayered — PrimeNG's runtime theme, every Angular component style and `styles.scss` — and unlayered CSS beats any layer whatever the specificity. So a utility silently does nothing when an unlayered rule sets the same property on the same element: a PrimeNG rule on a `styleClass` host (`p-button`, `p-message`, `p-select`, …), or a component rule like `.foo { margin: 0 }` next to `class="foo mt-4"`. Move the property into one place, or use the important modifier (`mt-4!`) — which also stops PrimeNG's keyframe animations (e.g. the p-message leave animation) from moving that property. `e2e/ui-tokens.spec.ts` guards both the token values and that layering.

## Component Structure

**Frontend (Angular):** every component lives in its own folder — `feature-name/feature-name.component.ts`. Subcomponents go in nested folders: `feature-name/sub-part/sub-part.component.ts`. Colocate specs, services, and related files in the same folder as the component they belong to. Decompose large components into focused subcomponents rather than growing monolithic templates.

**Backend (Go):** vertical slices in `internal/features/<domain>/`. Group by domain — service, model, platform-specific files, and tests all colocate in the same package. Add a new package when a new domain emerges; don't force single-file packages but keep each package focused on one concern.

## State Across Navigation

Use module-level `let` variables, not component properties — components are destroyed on navigate. → See `fix.component.ts:13-15`.
