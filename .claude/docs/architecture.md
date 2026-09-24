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

Browser mode: `wails.service.ts` detects when the Wails runtime is absent (Playwright / `ng serve`) and returns `BROWSER_MODE_DEFAULTS` for settings and "not installed" for the Claude Code status. There is no provider fallback in the frontend — `TextEnhancementService` passes straight through to `WailsService`, so nothing under `frontend/src` calls a provider (#87).

## Key Management

Priority chain (checked in this order): environment variable → OS keyring (`go-keyring`) → empty string.

Env var mapping (`internal/features/settings/service.go`):
- `openai` → `OPENAI_API_KEY`
- `claude` → `ANTHROPIC_API_KEY`
- `bedrock` → `AWS_SECRET_ACCESS_KEY`

Keys are NOT stored in `settings.json`. `GetKey(provider)` / `SetKey` / `DeleteKey` / `GetKeyStatus` are the API surface.

**Model selection:** `settings.json` carries `models` — per provider, a model for `fix` and one for `pyramidize`. An absent key or an empty string means the built-in default from `internal/llm/models.go`, so an older file needs no migration. Resolution order is request override (Pyramidize's panel) → settings → default. `SettingsService.ListModels(provider)` returns what a provider can serve and never fails the caller — `ModelList.Source` says what happened instead: `live`, `empty` (answered with nothing, e.g. a fresh `ollama serve`), `unusable` (listed models, none callable at `/chat/completions`), `unreachable` and `no-credentials` (these three fall back to the curated list), or `fixed` (the Claude Code CLI, which has no endpoint to ask). The UI has a sentence per case, and the cache TTL follows the source — 10 minutes for `live` and `fixed`, 30 seconds for everything else, which all describe something the user is in the middle of fixing.

## Platform Differences

**Shortcut:** `internal/features/shortcut/service_linux.go` (build tag `!windows`) is a no-op with a `Simulate()` helper for dev. `service_windows.go` uses Win32 `RegisterHotKey`.

**Clipboard:** `clipboard.Write()` on Linux requires `xsel` or `xclip` installed; failure is silently swallowed (best-effort).

**AI API calls:** Must go through the Go backend on Linux — WebKit2GTK blocks external HTTPS fetch from the webview. Every provider call lives in `internal/llm`: `Client.Complete(ctx, Request) (Response, error)` — `Request.JSONSchema` constrains the reply and each provider enforces it in its own dialect (`response_format: json_schema` for OpenAI and Ollama, `output_config.format` for Anthropic, `--json-schema` for the Claude Code CLI); a caller still parses defensively — with a registry keyed by provider ID (`openai`, `claude`, `claude-code`, `ollama`) and `llm.Config` supplying the API key, base URL and HTTP client. The HTTP providers are the vendor SDKs — `anthropic-sdk-go` for Claude, `openai-go` for OpenAI and for Ollama through its OpenAI-compatible `/v1` endpoint; the SDKs own retries and error types. For those HTTP providers, a provider's error body never reaches an error string (#41): errors carry provider, status and a reason KeyLint worded, and the body goes to `Debug` through `Redact`. The `claude-code` provider still puts CLI output into its errors — that path was out of scope for #41. The SDK clients take no configuration from the environment: Anthropic via `WithoutEnvironmentDefaults`, OpenAI — which has no equivalent — via an always-explicit base URL plus deletion of the headers `OPENAI_ORG_ID`/`OPENAI_PROJECT_ID`/`OPENAI_CUSTOM_HEADERS` would set, so no `*_BASE_URL` can redirect a request and no environment header rides along. That covers what those SDK versions read; an SDK bump can add a variable, which is why the tests assert on what reaches an httptest server rather than on the options we pass. The SDKs also reject a 2xx that is not `application/json`, which the hand-rolled clients tolerated. Features (`enhance`, `pyramidize`) build a `Request` and never touch a provider API directly.

## Dark Mode

`<body class="app-dark">` set in `frontend/src/index.html:10` before Angular bootstraps — eliminates flash. `ShellComponent.applyTheme()` never removes it: only the dark theme is styled, so `theme_preference` is not read until the light theme lands (#24, #25). PrimeNG Aura has gaps in dark mode CSS; manual overrides are in `frontend/src/styles.scss`.

## Routes

```
/welcome              → WelcomeWizardComponent (first-run guard redirects here)
/ → /fix              → FixComponent (default, silent clipboard fix)
/enhance              → TextEnhancementComponent (manual input/output)
/settings             → SettingsComponent
/dev-tools            → DevToolsComponent (dev mode only)
```

`firstRunGuard` in `app.routes.ts` checks `WailsService.isFirstRun()` before allowing shell routes.
