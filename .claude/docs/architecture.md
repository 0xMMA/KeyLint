# Architecture

## Service Wiring (Go)

Wire DI flow: `internal/app/wire.go` declares providers → `wire gen ./internal/app/` writes `wire_gen.go` → `main.go` calls `app.InitializeApp(wailsApp)`.

Services registered with Wails (callable from JS via `main.go:60-73`):
- `Settings`, `Welcome`, `Clipboard` — via Wire-injected `App` struct
- `EnhanceService` — wraps `Settings` for AI calls
- `LoggerService` — forwards frontend log messages to `debug.log`
- `UpdaterService` — version check + GitHub Releases API
- `simulateService` — local struct in `main.go`, exposes `SimulateShortcut` for dev-tools
- Registration pattern: `wailsApp.RegisterService(application.NewService(svc))`

After adding or changing a Go service method signature: run `wire gen ./internal/app/` then `wails3 generate bindings`. Both steps are required — Wire for DI, bindings for the JS RPC layer.

## RPC Bridge

`frontend/src/app/core/wails.service.ts` is the single point of contact between Angular and Go. It imports from `frontend/bindings/` (auto-generated). All components depend on `WailsService`, never on bindings directly.

Browser-mode fallback: `wails.service.ts` detects when Wails runtime is absent (Playwright / `ng serve`) and returns `BROWSER_MODE_DEFAULTS` for settings. `TextEnhancementService` falls back to direct fetch with `localStorage._e2e_apikey_claude` as the key.

## Key Management

Priority chain (checked in this order): environment variable → OS keyring (`go-keyring`) → empty string.

Env var mapping (`internal/features/settings/service.go`):
- `openai` → `OPENAI_API_KEY`
- `claude` → `ANTHROPIC_API_KEY`
- `bedrock` → `AWS_SECRET_ACCESS_KEY`

Keys are NOT stored in `settings.json`. `GetKey(provider)` / `SetKey` / `DeleteKey` / `GetKeyStatus` are the API surface.

## Platform Differences

**Shortcut:** `internal/features/shortcut/service_linux.go` (build tag `!windows`) is a no-op with a `Simulate()` helper for dev. `service_windows.go` uses Win32 `RegisterHotKey`.

**Clipboard:** `clipboard.Write()` on Linux requires `xsel` or `xclip` installed; failure is silently swallowed (best-effort).

**AI API calls:** Must go through Go backend (`internal/features/enhance/service.go`) on Linux. WebKit2GTK blocks external HTTPS fetch from the webview. The browser fallback in `TextEnhancementService` is only for Playwright E2E tests.

## Dark Mode

`<body class="app-dark">` set in `frontend/src/index.html:10` before Angular bootstraps — eliminates flash. `ShellComponent.applyTheme()` removes the class only if `theme_preference === 'light'` (dark-first logic). PrimeNG Aura has gaps in dark mode CSS; manual overrides are in `frontend/src/styles.scss`.

## Routes

```
/welcome              → WelcomeWizardComponent (first-run guard redirects here)
/ → /fix              → FixComponent (default, silent clipboard fix)
/enhance              → TextEnhancementComponent (manual input/output)
/settings             → SettingsComponent
/dev-tools            → DevToolsComponent (dev mode only)
```

`firstRunGuard` in `app.routes.ts` checks `WailsService.isFirstRun()` before allowing shell routes.
