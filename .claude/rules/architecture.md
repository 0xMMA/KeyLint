# Architecture Rules

## Service Registration

After adding or changing a Go service: run `wire gen ./internal/app/` then `wails3 generate bindings` — both required before building.

Registration pattern in `main.go`: `wailsApp.RegisterService(application.NewService(svc))` — always wrap with `application.NewService()`.

## RPC Bridge

All frontend→Go calls go through `WailsService` (`frontend/src/app/core/wails.service.ts`). Components never import from `frontend/bindings/` directly.

## AI API Calls

AI requests must go through the Go backend (`internal/features/enhance/service.go`). The browser-mode fallback in `TextEnhancementService` is only for Playwright E2E tests.

## Dark Mode

Dark-first: `<body class="app-dark">` in `index.html`. Only explicit `'light'` preference removes it. Never use `window.matchMedia` — returns light in jsdom.

## PrimeNG v21

Import individual symbols: `import { Tabs, TabList, Tab } from 'primeng/tabs'` — not module classes.

## Component Structure

**Frontend (Angular):** every component lives in its own folder — `feature-name/feature-name.component.ts`. Subcomponents go in nested folders: `feature-name/sub-part/sub-part.component.ts`. Colocate specs, services, and related files in the same folder as the component they belong to. Decompose large components into focused subcomponents rather than growing monolithic templates.

**Backend (Go):** vertical slices in `internal/features/<domain>/`. Group by domain — service, model, platform-specific files, and tests all colocate in the same package. Add a new package when a new domain emerges; don't force single-file packages but keep each package focused on one concern.

## State Across Navigation

Use module-level `let` variables, not component properties — components are destroyed on navigate. → See `fix.component.ts:13-15`.
