# Unified Logging — Design Spec

**Date:** 2026-04-05
**Status:** Approved
**Scope:** Logger package, settings model, settings UI, CLI flags, frontend log bridge, sensitive audit

## Problem

CLI commands (`-fix`, `-pyramidize`) have no logging. The GUI exposes only a boolean debug toggle — no level granularity. Frontend log messages are not clearly distinguished from backend messages. Some call sites may leak sensitive data through non-sensitive log functions.

## Design

### Log Levels

Six levels, ordered by severity:

| Level   | slog mapping       | Description                        |
|---------|--------------------|------------------------------------|
| off     | discard            | No logging (default)               |
| trace   | `slog.Level(-8)`   | Verbose internals, hot-path detail |
| debug   | `slog.LevelDebug`  | Diagnostic info for developers     |
| info    | `slog.LevelInfo`   | Normal operational events          |
| warning | `slog.LevelWarn`   | Recoverable issues                 |
| error   | `slog.LevelError`  | Failures requiring attention       |

### Logger Package (`internal/logger/logger.go`)

- `Init(level string, sensitive bool)` — replaces `Init(enabled bool, sensitive bool)`
- `"off"` discards all output. Any other valid level opens `debug.log` and sets the slog handler minimum to that level.
- New function: `Trace(msg string, args ...any)`
- All existing functions unchanged: `Debug`, `Info`, `Warn`, `Error`, `Sensitive`
- `Sensitive()` remains gated by the `sensitiveEnabled` bool, independent of log level. When sensitive is enabled, messages log at debug level.
- Define `const LevelTrace = slog.Level(-8)` for the custom trace level.
- Add `source` attribute: backend functions add `"source", "backend"`, frontend bridge adds `"source", "frontend"`.

### Settings Model (`internal/features/settings/model.go`)

- Replace `DebugLogging bool` with `LogLevel string` (json tag: `"log_level"`)
- `SensitiveLogging bool` unchanged
- `Default()` returns `LogLevel: "off"`
- **Migration:** Settings load checks for legacy `debug_logging` field. If present and `true` and `LogLevel` is empty → set `LogLevel = "debug"`. The old field is dropped on next save (Go unmarshals only known fields, so the old key is silently ignored after the migration).

### Settings Service

- `logger.Init(cfg.LogLevel, cfg.SensitiveLogging)` in both `main.go` (GUI path) and CLI path.

### Settings UI

- Replace the "Debug Logging" boolean toggle with a PrimeNG `Select` dropdown.
- Options: Off, Trace, Debug, Info, Warning, Error.
- "Sensitive Logging" toggle remains below, disabled when level is "off".
- Hint text: "Writes to ~/.config/KeyLint/debug.log (Linux) or %AppData%/KeyLint/debug.log (Windows)"

### CLI (`--log <level>`)

- Add `--log` flag to both `-fix` and `-pyramidize` via a shared `addLogFlag(fs *flag.FlagSet) *string` helper in `cli.go`.
- Default: `"off"`. Accepts: `off`, `trace`, `debug`, `info`, `warning`, `error`.
- Calls `logger.Init(level, false)` before executing the command.
- Sensitive is always `false` in CLI mode — prevents accidental credential leaks to terminal/pipes.
- Invalid level values produce a clear error message listing valid options.

### Frontend Log Bridge (`internal/features/logger/service.go`)

- `Log(level, msg string)` routes through the same logger functions but adds `"source", "frontend"` attribute.
- Supports: `"trace"`, `"debug"`, `"info"`, `"warn"`, `"error"`.
- Frontend errors (Angular `ErrorHandler`, uncaught exceptions) should call this service so they appear in `debug.log`.

### Backend Log Tagging

- All backend `logger.*()` calls include `"source", "backend"` as a default attribute set on the handler, so individual call sites don't need to add it manually.
- Frontend bridge overrides with `"source", "frontend"`.

Implementation: use `slog.Handler` wrapping or `logger.With("source", "backend")` as the default logger instance. The frontend service creates calls with `"source", "frontend"` via a separate logger instance or by passing the attribute explicitly.

### Sensitive Audit

Existing call sites reviewed:

| File | Call | Verdict |
|------|------|---------|
| `enhance/service.go:65` | `Info("enhance: start", "provider", ..., "input_len", ...)` | Safe — no user text, only length |
| `enhance/service.go:107,118,140,152,174,184` | `Sensitive(...)` | Correct — payloads gated |
| `pyramidize/api_*.go` | `Sensitive(...)` | Correct — payloads gated |
| `pyramidize/service.go:97` | `Info("pyramidize: start", "docType", ..., "provider", ...)` | Safe — metadata only |
| `clipboard/service.go:39,54` | `Debug("clipboard: read/write", "len", ...)` | Safe — only length |
| `features/logger/service.go` | `Debug/Info/Warn/Error("frontend: " + msg)` | **Needs review** — raw frontend messages could contain user text. Route through `Sensitive()` or document that frontend messages may contain user content. |

**Decision:** Not all frontend log messages are sensitive — error stack traces and component lifecycle events are safe, but info/debug/trace messages may contain user text. The bridge should:
- Route `"error"` and `"warn"` level messages through normal `logger.Error/Warn` (with `source=frontend`) — these are operational.
- Route `"info"`, `"debug"`, `"trace"` through `logger.Sensitive` — these may contain user text and should only appear when sensitive logging is explicitly enabled.

## File Changes Summary

| File | Change |
|------|--------|
| `internal/logger/logger.go` | New `Init` signature, add `Trace`, add `LevelTrace`, default `source=backend` attribute |
| `internal/features/logger/service.go` | Add `source=frontend` attribute, gate info/debug/trace through `Sensitive` |
| `internal/features/settings/model.go` | `DebugLogging bool` → `LogLevel string`, migration logic |
| `internal/features/settings/service.go` | Migration on load |
| `internal/cli/cli.go` | `addLogFlag` helper, `initLogger` helper |
| `internal/cli/fix.go` | Add `--log` flag |
| `internal/cli/pyramidize.go` | Add `--log` flag |
| `main.go` | `logger.Init(cfg.LogLevel, cfg.SensitiveLogging)` |
| `frontend/.../settings.component.ts` | Replace toggle with Select dropdown |
| `docs/logging.md` | Requirements and conventions reference |
| `CLAUDE.md` | One-line reference to `docs/logging.md` |
