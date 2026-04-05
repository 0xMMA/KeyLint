# Unified Logging — Design Spec

**Date:** 2026-04-05
**Status:** Approved
**Scope:** Logger package, settings model, settings UI, CLI flags, frontend log bridge, sensitive redaction, testing

## Problem

CLI commands (`-fix`, `-pyramidize`) have no logging. The GUI exposes only a boolean debug toggle — no level granularity. Frontend log messages are not clearly distinguished from backend messages. Sensitive data (user text, API payloads, credentials) can leak through log calls that don't gate on the sensitive flag.

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

### Sensitive Redaction via `slog.LogValuer`

Remove the standalone `Sensitive()` function. Replace with a `Redact(value any) slog.LogValuer` wrapper that uses slog's native `LogValuer` interface:

```go
// Definition in internal/logger/logger.go
const LevelTrace = slog.Level(-8)

type redacted struct{ v any }

func (r redacted) LogValue() slog.Value {
    if !sensitiveEnabled {
        return slog.StringValue("[redacted]")
    }
    return slog.AnyValue(r.v)
}

// Redact wraps a value so it self-redacts when sensitive logging is off.
func Redact(v any) slog.LogValuer { return redacted{v} }
```

**Call-site convention:** any value that could contain user text, API payloads, or credentials is wrapped in `Redact()`. The event message and safe metadata (provider name, status code, byte length) are never wrapped.

```go
// Before (old Sensitive function — entire entry hidden):
logger.Sensitive("enhance: request", "provider", "openai", "payload", string(body))

// After (event always visible, payload redacted unless sensitive is on):
logger.Debug("enhance: request", "provider", "openai", "payload", logger.Redact(string(body)))
```

This means:
- Every log entry appears at its configured level — you always see *what happened*
- Only the sensitive *values* get redacted — safe metadata stays visible
- No external dependencies — uses stdlib `slog.LogValuer` contract
- The old `Sensitive()` function is removed entirely

### Logger Package (`internal/logger/logger.go`)

- `Init(level string, sensitive bool)` — replaces `Init(enabled bool, sensitive bool)`
- `"off"` discards all output. Any other valid level opens `debug.log` and sets the slog handler minimum to that level.
- New function: `Trace(msg string, args ...any)` using `LevelTrace = slog.Level(-8)`
- Existing functions unchanged: `Debug`, `Info`, `Warn`, `Error`
- `Sensitive()` removed — replaced by `Redact()` (see above)
- Default logger instance carries `"source", "backend"` attribute via `slog.Logger.With()`
- Expose `Frontend() *slog.Logger` or a `FrontendInfo/Debug/Warn/Error` set that carries `"source", "frontend"`

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

- `Log(level, msg string)` routes through logger functions with `"source", "frontend"` attribute.
- Supports: `"trace"`, `"debug"`, `"info"`, `"warn"`, `"error"`.
- `msg` is wrapped in `Redact()` for trace/debug/info levels — these may contain user text. Error and warn messages pass through unredacted since they are operational (stack traces, lifecycle warnings).
- Frontend errors (Angular `ErrorHandler`, uncaught exceptions) should call this service so they appear in `debug.log`.

### Backend Log Tagging

- Default backend logger instance carries `"source", "backend"` via `slog.Logger.With("source", "backend")`.
- Frontend bridge uses a separate logger instance with `"source", "frontend"`.
- Individual call sites don't need to add the source attribute — it's baked into the logger instance.

### Sensitive Audit — All Call Sites

Every existing call site must be reviewed. The rule: **if the value could contain user text, API payloads, or credentials, wrap it in `Redact()`.**

| File | Call | Action |
|------|------|--------|
| `enhance/service.go:65` | `Info("enhance: start", "provider", ..., "input_len", ...)` | No change — metadata only |
| `enhance/service.go:107,118` | `Sensitive("enhance: request/response", "payload", ...)` | → `Debug(...)` with `Redact(payload)` |
| `enhance/service.go:140,152` | `Sensitive("enhance: request/response", "payload", ...)` | → `Debug(...)` with `Redact(payload)` |
| `enhance/service.go:174,184` | `Sensitive("enhance: request/response", "payload", ...)` | → `Debug(...)` with `Redact(payload)` |
| `pyramidize/api_claude.go:31,48` | `Sensitive(... "len", len(payload))` | → `Debug(...)` with `Redact(payload)` (log the body, not just length) |
| `pyramidize/api_openai.go:36,52` | `Sensitive(...)` | → `Debug(...)` with `Redact(payload)` |
| `pyramidize/api_ollama.go:37,52` | `Sensitive(...)` | → `Debug(...)` with `Redact(payload)` |
| `pyramidize/service.go:97` | `Info("pyramidize: start", "docType", ..., "provider", ...)` | No change — metadata only |
| `clipboard/service.go:39,54` | `Debug("clipboard: read/write", "len", ...)` | No change — only length |
| `features/logger/service.go` | `Debug/Info/Warn/Error("frontend: " + msg)` | → Wrap `msg` in `Redact()` |
| `main.go:120` | `Info("shortcut: registered", "key", cfg.ShortcutKey)` | No change — config value, not user text |
| All other `Info/Debug/Warn/Error` calls | Metadata, status codes, error messages | No change — no user content |

## Testing Strategy

The logger and redaction system must be thoroughly tested. No mocks for the core logger — test real output.

### Logger Package Tests (`internal/logger/logger_test.go`)

**Init & level filtering:**
- `Init("off", false)` → no output produced for any level
- `Init("error", false)` → only error-level messages appear
- `Init("warning", false)` → warning + error appear, info/debug/trace do not
- `Init("info", false)` → info + warning + error appear
- `Init("debug", false)` → debug + info + warning + error appear
- `Init("trace", false)` → all levels appear including trace
- Invalid level string → falls back to off (or returns error)

**Redact behavior:**
- `Redact(value)` with `sensitive=false` → output contains `[redacted]`, NOT the value
- `Redact(value)` with `sensitive=true` → output contains the actual value
- `Redact(value)` with various types: string, []byte, struct, nil
- Verify redaction works through the full slog pipeline (write to buffer, parse output, assert)
- **Negative test:** log a "secret" value via `Redact()`, read back the log file, assert the secret string is NOT present when sensitive is off

**Source tagging:**
- Backend logger output contains `source=backend`
- Frontend logger output contains `source=frontend`

**Trace level:**
- `Trace()` emits at `LevelTrace` (-8)
- Trace messages appear when level is "trace", not when level is "debug"

**File output:**
- Init creates `debug.log` in the expected directory
- Multiple Init calls don't leak file handles (close previous file)

### Settings Migration Tests (`internal/features/settings/`)

- Legacy JSON with `"debug_logging": true` and no `"log_level"` → loads as `LogLevel: "debug"`
- Legacy JSON with `"debug_logging": false` → loads as `LogLevel: "off"`
- New JSON with `"log_level": "warning"` → loads correctly
- JSON with both fields → `log_level` takes precedence
- Round-trip: load legacy → save → reload → `log_level` present, `debug_logging` absent

### CLI Flag Tests (`internal/cli/`)

- `--log debug` → logger initialised at debug level
- `--log off` (or omitted) → no log output
- `--log invalid` → error message listing valid levels
- `--log` flag works for both `-fix` and `-pyramidize`
- Sensitive is always off in CLI mode regardless of flag

### Frontend Bridge Tests (`internal/features/logger/`)

- `Log("error", "something broke")` → appears in log with `source=frontend`
- `Log("info", "user typed X")` → msg content redacted when sensitive is off
- `Log("info", "user typed X")` → msg content visible when sensitive is on
- Unknown level defaults to info

### Settings UI Tests (Vitest)

- Select dropdown renders with all 6 options
- Changing selection calls the settings save method with correct `log_level` value
- Sensitive toggle is disabled when level is "off"
- Sensitive toggle is enabled when level is anything else

## File Changes Summary

| File | Change |
|------|--------|
| `internal/logger/logger.go` | New `Init(level, sensitive)`, add `Trace`, `Redact`, `LevelTrace`, remove `Sensitive`, `source=backend` default |
| `internal/logger/logger_test.go` | **New** — comprehensive tests for levels, redaction, source tagging, file output |
| `internal/features/logger/service.go` | `source=frontend`, wrap msg in `Redact()` |
| `internal/features/logger/service_test.go` | **New** — bridge tests for level routing, redaction, source tagging |
| `internal/features/settings/model.go` | `DebugLogging bool` → `LogLevel string`, update `Default()` |
| `internal/features/settings/service.go` | Migration logic on load |
| `internal/features/settings/service_test.go` | Migration tests (legacy → new, round-trip) |
| `internal/cli/cli.go` | `addLogFlag` helper |
| `internal/cli/fix.go` | Add `--log` flag, call `logger.Init` |
| `internal/cli/pyramidize.go` | Add `--log` flag, call `logger.Init` |
| `internal/cli/cli_test.go` | CLI flag tests for `--log` |
| `main.go` | `logger.Init(cfg.LogLevel, cfg.SensitiveLogging)` |
| `internal/features/enhance/service.go` | `Sensitive()` → `Debug()` + `Redact()` |
| `internal/features/pyramidize/api_claude.go` | `Sensitive()` → `Debug()` + `Redact()` |
| `internal/features/pyramidize/api_openai.go` | `Sensitive()` → `Debug()` + `Redact()` |
| `internal/features/pyramidize/api_ollama.go` | `Sensitive()` → `Debug()` + `Redact()` |
| `frontend/.../settings.component.ts` | Replace toggle with Select dropdown |
| `frontend/.../settings.component.spec.ts` | UI tests for dropdown + sensitive toggle state |
| `docs/logging.md` | Requirements and conventions reference |
| `CLAUDE.md` | One-line reference to `docs/logging.md` |
