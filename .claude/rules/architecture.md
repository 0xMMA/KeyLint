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

Dark only, for now: `<html class="app-dark">` in `index.html`, and `ShellComponent.applyTheme()` keeps it whatever `theme_preference` says — only the dark theme is styled (#24). The class goes on `<html>`, never `<body>`: the `--kl-*` tokens live on `:root` and some alias others, and an alias resolves where it is declared, so a theme class below the root would leave them on the old values (#28 was this, with PrimeNG's tokens). `theme.css` sets `color-scheme: dark` for native controls and scrollbars. The field stays in the settings model for the light theme (#25), which is where reading it comes back. Never use `window.matchMedia` for theme detection — jsdom does not define it at all, so a spec that reaches it throws rather than returning light.

## Shared UI

No component library: PrimeNG was removed in E11 after its relicensing (`docs/ui/primeng-replacement.md`) — don't add it, or another one, back. KeyLint's own components live in `frontend/src/app/shared/ui/<name>/` and are imported from `shared/ui` (index.ts): `kl-button`, `kl-message`, `kl-tag`, `kl-card`, `kl-spinner`, `kl-icon` (vendored Lucide SVGs in `icon/icons.ts`, add one there), `kl-switch`, `kl-checkbox`, `klInput`, `kl-segmented`, `kl-select` (with `editable`), `kl-tabs`, `klTooltip`. Overlays (select, tooltip) use the CDK overlay on `<body>` with `usePopover: false`; they follow, and close when their trigger leaves, only the scroll containers the CDK knows, so an element that scrolls and can hold one carries `cdkScrollable` (the shell's `.layout-main` and the Pyramidize options column do). Every state of each is on `/dev-tools/ui`. Form controls are ControlValueAccessors; testids go on the host, which wraps the real `<button>`/`<input>`.

## Styling: tokens and Tailwind

Colours, surfaces and radii come from the `--kl-*` tokens in `frontend/src/theme.css` (`var(--kl-muted)`, `var(--kl-line)`, …), whose values are `docs/ui/design.md`'s. New colour needed? Add a token there, don't hard-code it.

Layout utilities are Tailwind v4 (`mt-4`, `flex`, `gap-4`, `text-muted`, `bg-sheet`); Tailwind's spacing step is 0.25rem (`mt-4` = 1rem). Only KeyLint's colours exist as utilities. Tailwind's preflight reset is on (`@layer base`): no UA margins, padding or borders, border-box everywhere, form controls inherit the font, headings and lists unstyled, img/svg are blocks — so a component that wants a list marker, a heading size or a margin sets it itself. Links stay underlined (`styles.scss`). Utilities sit in `@layer utilities`, and every Angular component style and `styles.scss` is unlayered — and unlayered CSS beats any layer whatever the specificity. So a utility silently does nothing when a component rule sets the same property on the same element (`.foo { margin: 0 }` next to `class="foo mt-4"`). Move the property into one place, or use the important modifier (`mt-4!`). `e2e/ui-tokens.spec.ts` guards the token values, the preflight base and that layering.

## Component Structure

**Frontend (Angular):** every component lives in its own folder — `feature-name/feature-name.component.ts`. Subcomponents go in nested folders: `feature-name/sub-part/sub-part.component.ts`. Colocate specs, services, and related files in the same folder as the component they belong to. Decompose large components into focused subcomponents rather than growing monolithic templates.

**Backend (Go):** vertical slices in `internal/features/<domain>/`. Group by domain — service, model, platform-specific files, and tests all colocate in the same package. Add a new package when a new domain emerges; don't force single-file packages but keep each package focused on one concern.

## State Across Navigation

Use module-level `let` variables, not component properties — components are destroyed on navigate. → See `fix.component.ts:13-15`.
