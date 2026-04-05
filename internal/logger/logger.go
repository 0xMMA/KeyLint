// Package logger provides a thin wrapper around log/slog with level-based
// filtering, sensitive-value redaction, and source tagging (backend/frontend).
// It is disabled by default (all output discarded) and must be explicitly
// enabled via Init — typically driven by the LogLevel settings field.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// LevelTrace is a custom slog level below Debug.
const LevelTrace = slog.Level(-8)

// Package-level state.
var (
	l                = slog.New(slog.NewTextHandler(io.Discard, nil))
	baseH   slog.Handler = slog.NewTextHandler(io.Discard, nil)
	logFile          *os.File
	sensitiveEnabled bool
)

// levelNames maps level strings to slog.Level values.
var levelNames = map[string]slog.Level{
	"trace":   LevelTrace,
	"debug":   slog.LevelDebug,
	"info":    slog.LevelInfo,
	"warning": slog.LevelWarn,
	"error":   slog.LevelError,
}

// Init enables or disables structured logging. The level parameter is one of
// "trace", "debug", "info", "warning", "error", or "off". An unrecognised
// level is treated as "off". When a valid level is provided, output goes to
// ~/.config/KeyLint/debug.log (or the platform equivalent).
// sensitive controls whether Redact() reveals the underlying value.
func Init(level string, sensitive bool) {
	sensitiveEnabled = sensitive

	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}

	lvl, ok := levelNames[level]
	if !ok {
		// "off" or invalid → discard
		baseH = slog.NewTextHandler(io.Discard, nil)
		l = slog.New(baseH)
		return
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		baseH = slog.NewTextHandler(io.Discard, nil)
		l = slog.New(baseH)
		return
	}
	dir = filepath.Join(dir, "KeyLint")
	_ = os.MkdirAll(dir, 0700)
	logPath := filepath.Join(dir, "debug.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		baseH = slog.NewTextHandler(io.Discard, nil)
		l = slog.New(baseH)
		return
	}
	logFile = f

	replaceFunc := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.LevelKey {
			if a.Value.Any().(slog.Level) == LevelTrace {
				a.Value = slog.StringValue("TRACE")
			}
		}
		return a
	}
	baseH = slog.NewTextHandler(f, &slog.HandlerOptions{
		Level:       lvl,
		ReplaceAttr: replaceFunc,
	})
	l = slog.New(baseH).With("source", "backend")
	l.Info("logger initialized", "path", logPath, "sensitive", sensitive)
}

// initWithWriter is the same as Init but writes to w instead of a log file.
// Used by tests.
func initWithWriter(w io.Writer, level string, sensitive bool) {
	sensitiveEnabled = sensitive

	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}

	lvl, ok := levelNames[level]
	if !ok {
		baseH = slog.NewTextHandler(io.Discard, nil)
		l = slog.New(baseH)
		return
	}

	replaceFunc := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.LevelKey {
			if a.Value.Any().(slog.Level) == LevelTrace {
				a.Value = slog.StringValue("TRACE")
			}
		}
		return a
	}
	baseH = slog.NewTextHandler(w, &slog.HandlerOptions{
		Level:       lvl,
		ReplaceAttr: replaceFunc,
	})
	l = slog.New(baseH).With("source", "backend")
}

// FrontendLogger returns a logger tagged with source=frontend.
// It uses baseH (no source attr baked in) to avoid double source attributes.
func FrontendLogger() *slog.Logger {
	return slog.New(baseH).With("source", "frontend")
}

// --- Standard log functions ---

// Trace logs at the custom TRACE level (below DEBUG).
func Trace(msg string, args ...any) {
	l.Log(context.Background(), LevelTrace, msg, args...)
}

// Debug logs at DEBUG level.
func Debug(msg string, args ...any) { l.Debug(msg, args...) }

// Info logs at INFO level.
func Info(msg string, args ...any) { l.Info(msg, args...) }

// Warn logs at WARN level.
func Warn(msg string, args ...any) { l.Warn(msg, args...) }

// Error logs at ERROR level.
func Error(msg string, args ...any) { l.Error(msg, args...) }

// --- Redaction ---

// redacted wraps a value and implements slog.LogValuer. When sensitive logging
// is disabled, it resolves to "[redacted]" instead of the underlying value.
type redacted struct{ v any }

// LogValue implements slog.LogValuer.
func (r redacted) LogValue() slog.Value {
	if sensitiveEnabled {
		return slog.AnyValue(r.v)
	}
	return slog.StringValue("[redacted]")
}

// Redact wraps v so that it is shown as "[redacted]" in log output unless
// sensitive logging is enabled. Use for API keys, request bodies, etc.
func Redact(v any) slog.LogValuer {
	return redacted{v: v}
}

// --- Backward compatibility (will be removed in Task 5) ---

// Sensitive logs only when sensitive logging is enabled.
// Deprecated: use Debug(msg, "key", Redact(value)) instead.
func Sensitive(msg string, args ...any) {
	if sensitiveEnabled {
		l.Debug(msg, args...)
	}
}
