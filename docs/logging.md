# Logging Conventions

## Log Levels

| Level   | When to use                                    |
|---------|------------------------------------------------|
| off     | Default. No logging.                           |
| trace   | Verbose internals, hot-path detail              |
| debug   | Diagnostic info for developers                  |
| info    | Normal operational events                       |
| warning | Recoverable issues                              |
| error   | Failures requiring attention                    |

## Sensitive Redaction

**Rule:** If a log value could contain user text, API payloads, or credentials, wrap it in `logger.Redact()`.

```go
// Safe metadata — no wrapping needed
logger.Info("enhance: start", "provider", cfg.ActiveProvider, "input_len", len(text))

// Sensitive data — wrap in Redact()
logger.Debug("enhance: request", "provider", "openai", "payload", logger.Redact(string(body)))
```

When `SensitiveLogging` is off, `Redact()` outputs `[redacted]`. When on, the real value is shown. Uses slog's native `LogValuer` interface.

Never wrap: provider names, status codes, byte lengths, error messages, config keys.
Always wrap: API request/response bodies, user text, clipboard content, API keys.

## Source Tagging

All log entries include a `source` attribute:
- `source=backend` — Go backend (automatic via default logger instance)
- `source=frontend` — Angular frontend (via the log bridge service)

## CLI Usage

```bash
./bin/KeyLint -fix --log debug "text to fix"
./bin/KeyLint -pyramidize --log info -f input.md
```

Valid levels: `off`, `trace`, `debug`, `info`, `warning`, `error`. Default: `off`.
Sensitive logging is always off in CLI mode to prevent credential leaks to terminal.

## Settings

- **UI:** Settings > General > Log Level dropdown (Off/Trace/Debug/Info/Warning/Error)
- **JSON field:** `"log_level": "off"` (replaces legacy `"debug_logging": true/false`)
- **Migration:** Legacy `debug_logging: true` is auto-migrated to `log_level: "debug"` on load
