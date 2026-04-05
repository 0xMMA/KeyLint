# Unified Logging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the boolean debug logging toggle with a six-level log system, add `Redact()` for sensitive value protection via slog's `LogValuer`, add `--log` CLI flag, tag frontend vs backend log sources, and update the settings UI to a dropdown.

**Architecture:** The `internal/logger` package becomes the single source of truth for log level, sensitivity, and redaction. `Redact()` wraps values with a `slog.LogValuer` that self-redacts when sensitive logging is off. Settings model migrates `debug_logging: bool` → `log_level: string`. CLI commands get a shared `--log <level>` flag. Frontend log bridge tags messages with `source=frontend` and wraps message content in `Redact()`.

**Tech Stack:** Go 1.26 `log/slog`, Angular v21, PrimeNG v21 `Select`, Vitest

---

### Task 1: Logger Package — Levels, Trace, Redact, Source Tagging

**Files:**
- Modify: `internal/logger/logger.go` (full rewrite)
- Create: `internal/logger/logger_test.go`

- [ ] **Step 1: Write the failing tests for level filtering**

Create `internal/logger/logger_test.go`:

```go
package logger

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// initWithBuffer sets up the logger to write to a buffer at the given level.
// Returns the buffer so tests can inspect output.
func initWithBuffer(t *testing.T, level string, sensitive bool) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	initWithWriter(&buf, level, sensitive)
	t.Cleanup(func() {
		l = slog.New(slog.NewTextHandler(io.Discard, nil))
		sensitiveEnabled = false
	})
	return &buf
}

func TestInit_Off_ProducesNoOutput(t *testing.T) {
	buf := initWithBuffer(t, "off", false)
	Trace("trace msg")
	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")
	if buf.Len() != 0 {
		t.Errorf("expected no output with level=off, got:\n%s", buf.String())
	}
}

func TestInit_Error_OnlyErrorAppears(t *testing.T) {
	buf := initWithBuffer(t, "error", false)
	Info("info msg")
	Warn("warn msg")
	Error("error msg")
	output := buf.String()
	if strings.Contains(output, "info msg") {
		t.Error("info should not appear at error level")
	}
	if strings.Contains(output, "warn msg") {
		t.Error("warn should not appear at error level")
	}
	if !strings.Contains(output, "error msg") {
		t.Error("error should appear at error level")
	}
}

func TestInit_Warning_WarnAndErrorAppear(t *testing.T) {
	buf := initWithBuffer(t, "warning", false)
	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")
	output := buf.String()
	if strings.Contains(output, "debug msg") {
		t.Error("debug should not appear at warning level")
	}
	if strings.Contains(output, "info msg") {
		t.Error("info should not appear at warning level")
	}
	if !strings.Contains(output, "warn msg") {
		t.Error("warn should appear at warning level")
	}
	if !strings.Contains(output, "error msg") {
		t.Error("error should appear at warning level")
	}
}

func TestInit_Info_InfoWarnErrorAppear(t *testing.T) {
	buf := initWithBuffer(t, "info", false)
	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")
	output := buf.String()
	if strings.Contains(output, "debug msg") {
		t.Error("debug should not appear at info level")
	}
	if !strings.Contains(output, "info msg") {
		t.Error("info should appear")
	}
	if !strings.Contains(output, "warn msg") {
		t.Error("warn should appear")
	}
	if !strings.Contains(output, "error msg") {
		t.Error("error should appear")
	}
}

func TestInit_Debug_AllExceptTraceAppear(t *testing.T) {
	buf := initWithBuffer(t, "debug", false)
	Trace("trace msg")
	Debug("debug msg")
	Info("info msg")
	output := buf.String()
	if strings.Contains(output, "trace msg") {
		t.Error("trace should not appear at debug level")
	}
	if !strings.Contains(output, "debug msg") {
		t.Error("debug should appear")
	}
	if !strings.Contains(output, "info msg") {
		t.Error("info should appear")
	}
}

func TestInit_Trace_AllLevelsAppear(t *testing.T) {
	buf := initWithBuffer(t, "trace", false)
	Trace("trace msg")
	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")
	output := buf.String()
	for _, want := range []string{"trace msg", "debug msg", "info msg", "warn msg", "error msg"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output", want)
		}
	}
}

func TestInit_InvalidLevel_FallsBackToOff(t *testing.T) {
	buf := initWithBuffer(t, "banana", false)
	Info("should not appear")
	if buf.Len() != 0 {
		t.Errorf("invalid level should fall back to off, got:\n%s", buf.String())
	}
}
```

- [ ] **Step 2: Write the failing tests for Redact**

Append to `internal/logger/logger_test.go`:

```go
func TestRedact_SensitiveOff_RedactsValue(t *testing.T) {
	buf := initWithBuffer(t, "debug", false) // sensitive=false
	Debug("test", "secret", Redact("my-api-key-123"))
	output := buf.String()
	if strings.Contains(output, "my-api-key-123") {
		t.Error("secret value should be redacted when sensitive=false")
	}
	if !strings.Contains(output, "[redacted]") {
		t.Error("expected [redacted] placeholder")
	}
}

func TestRedact_SensitiveOn_ShowsValue(t *testing.T) {
	buf := initWithBuffer(t, "debug", true) // sensitive=true
	Debug("test", "secret", Redact("my-api-key-123"))
	output := buf.String()
	if !strings.Contains(output, "my-api-key-123") {
		t.Error("secret value should be visible when sensitive=true")
	}
	if strings.Contains(output, "[redacted]") {
		t.Error("should not contain [redacted] when sensitive=true")
	}
}

func TestRedact_NilValue(t *testing.T) {
	buf := initWithBuffer(t, "debug", false)
	Debug("test", "val", Redact(nil))
	output := buf.String()
	if !strings.Contains(output, "[redacted]") {
		t.Error("nil value should still show [redacted]")
	}
}

func TestRedact_ByteSlice(t *testing.T) {
	buf := initWithBuffer(t, "debug", true)
	Debug("test", "body", Redact([]byte("secret bytes")))
	output := buf.String()
	if !strings.Contains(output, "secret bytes") {
		t.Error("byte slice should be visible when sensitive=true")
	}
}

func TestRedact_NegativeTest_SecretNotInLogFile(t *testing.T) {
	// The most critical test: with sensitive OFF, the secret must NOT
	// appear anywhere in the log output — not in any form.
	secret := "SUPER_SECRET_KEY_abc123xyz"
	buf := initWithBuffer(t, "trace", false) // lowest level, sensitive OFF
	Trace("api call", "key", Redact(secret))
	Debug("request", "payload", Redact(`{"auth":"` + secret + `"}`))
	Info("response", "body", Redact(secret))
	output := buf.String()
	if strings.Contains(output, secret) {
		t.Fatalf("SECRET LEAKED in log output:\n%s", output)
	}
}
```

- [ ] **Step 3: Write the failing tests for source tagging**

Append to `internal/logger/logger_test.go`:

```go
func TestBackendLogger_HasSourceBackend(t *testing.T) {
	buf := initWithBuffer(t, "info", false)
	Info("hello")
	output := buf.String()
	if !strings.Contains(output, "source=backend") {
		t.Errorf("expected source=backend in output, got:\n%s", output)
	}
}

func TestFrontendLogger_HasSourceFrontend(t *testing.T) {
	buf := initWithBuffer(t, "info", false)
	fl := FrontendLogger()
	fl.Info("hello from frontend")
	output := buf.String()
	if !strings.Contains(output, "source=frontend") {
		t.Errorf("expected source=frontend in output, got:\n%s", output)
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `cd /home/dev/projects/keylint && go test ./internal/logger/ -v -count=1`
Expected: compilation errors (initWithWriter, Redact, FrontendLogger, Trace not defined yet)

- [ ] **Step 5: Implement the logger package**

Rewrite `internal/logger/logger.go`:

```go
// Package logger provides a thin wrapper around log/slog with level-based
// filtering, sensitive value redaction via slog.LogValuer, and source tagging.
package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// LevelTrace is a custom slog level below Debug for verbose tracing.
const LevelTrace = slog.Level(-8)

var (
	l                = slog.New(slog.NewTextHandler(io.Discard, nil))
	logFile          *os.File
	sensitiveEnabled bool
)

// levelNames maps config strings to slog levels.
var levelNames = map[string]slog.Level{
	"trace":   LevelTrace,
	"debug":   slog.LevelDebug,
	"info":    slog.LevelInfo,
	"warning": slog.LevelWarn,
	"error":   slog.LevelError,
}

// Init configures the logger. level is one of: "off", "trace", "debug",
// "info", "warning", "error". Invalid values are treated as "off".
// sensitive controls whether Redact() shows or hides wrapped values.
func Init(level string, sensitive bool) {
	sensitiveEnabled = sensitive
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
	slogLevel, ok := levelNames[level]
	if !ok {
		// "off" or invalid → discard
		l = slog.New(slog.NewTextHandler(io.Discard, nil))
		return
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		l = slog.New(slog.NewTextHandler(io.Discard, nil))
		return
	}
	dir = filepath.Join(dir, "KeyLint")
	_ = os.MkdirAll(dir, 0700)
	logPath := filepath.Join(dir, "debug.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		l = slog.New(slog.NewTextHandler(io.Discard, nil))
		return
	}
	logFile = f
	l = slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slogLevel})).With("source", "backend")
	l.Info("logger initialized", "path", logPath, "level", level, "sensitive", sensitive)
}

// initWithWriter is used by tests to redirect output to a buffer.
func initWithWriter(w io.Writer, level string, sensitive bool) {
	sensitiveEnabled = sensitive
	slogLevel, ok := levelNames[level]
	if !ok {
		l = slog.New(slog.NewTextHandler(io.Discard, nil))
		return
	}
	l = slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slogLevel})).With("source", "backend")
}

// FrontendLogger returns a logger tagged with source=frontend.
// Used by the frontend log bridge service.
func FrontendLogger() *slog.Logger {
	// Build from the same handler as l, but swap the source attribute.
	// We create a child logger from the underlying handler to inherit the level.
	return l.With("source", "frontend")
}

func Trace(msg string, args ...any) { l.Log(nil, LevelTrace, msg, args...) }
func Debug(msg string, args ...any) { l.Debug(msg, args...) }
func Info(msg string, args ...any)  { l.Info(msg, args...) }
func Warn(msg string, args ...any)  { l.Warn(msg, args...) }
func Error(msg string, args ...any) { l.Error(msg, args...) }

// redacted wraps a value and implements slog.LogValuer to self-redact
// when sensitive logging is disabled.
type redacted struct{ v any }

func (r redacted) LogValue() slog.Value {
	if !sensitiveEnabled {
		return slog.StringValue("[redacted]")
	}
	return slog.AnyValue(r.v)
}

// Redact wraps a value so it self-redacts when sensitive logging is off.
// Use this for any value that could contain user text, API payloads, or credentials.
func Redact(v any) slog.LogValuer { return redacted{v} }
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd /home/dev/projects/keylint && go test ./internal/logger/ -v -count=1`
Expected: all tests PASS

- [ ] **Step 7: Fix FrontendLogger source tag issue**

The `FrontendLogger()` implementation above will produce `source=backend source=frontend` because `.With()` appends rather than replaces. We need to build the frontend logger from the raw handler instead. Update `FrontendLogger()`:

```go
// FrontendLogger returns a logger tagged with source=frontend.
func FrontendLogger() *slog.Logger {
	// Get the underlying handler and create a fresh logger with source=frontend.
	// This avoids the backend logger's source=backend attribute.
	h := l.Handler()
	return slog.New(h).With("source", "frontend")
}
```

And update `initWithWriter` to store the handler separately so FrontendLogger can use it without inheriting `source=backend`:

Actually, the cleaner approach is to store the base handler before adding the source attribute. Revise the implementation:

```go
var (
	l       = slog.New(slog.NewTextHandler(io.Discard, nil))
	baseH   slog.Handler = slog.NewTextHandler(io.Discard, nil)
	logFile *os.File
	sensitiveEnabled bool
)

// In Init and initWithWriter, store baseH before adding source:
// baseH = slog.NewTextHandler(w, &slog.HandlerOptions{Level: slogLevel})
// l = slog.New(baseH).With("source", "backend")

func FrontendLogger() *slog.Logger {
	return slog.New(baseH).With("source", "frontend")
}
```

Update both `Init` and `initWithWriter` accordingly. Then re-run the test for `TestFrontendLogger_HasSourceFrontend` — it should show `source=frontend` without `source=backend`.

- [ ] **Step 8: Run tests again to confirm fix**

Run: `cd /home/dev/projects/keylint && go test ./internal/logger/ -v -count=1`
Expected: all tests PASS, `TestFrontendLogger_HasSourceFrontend` shows only `source=frontend`

- [ ] **Step 9: Commit**

```bash
git add internal/logger/logger.go internal/logger/logger_test.go
git commit -m "feat(logger): add level-based logging, Redact via LogValuer, source tagging"
```

---

### Task 2: Settings Model — LogLevel Migration

**Files:**
- Modify: `internal/features/settings/model.go`
- Modify: `internal/features/settings/service.go`
- Modify: `internal/features/settings/service_test.go`

- [ ] **Step 1: Write the failing migration tests**

Append to `internal/features/settings/service_test.go`:

```go
func TestMigration_LegacyDebugLoggingTrue_BecomesDebug(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "KeyLint")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	// Write legacy JSON with debug_logging: true and no log_level
	legacy := `{"active_provider":"openai","debug_logging":true,"sensitive_logging":true}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	svc := newServiceAt(t, tmp)
	got := svc.Get()
	if got.LogLevel != "debug" {
		t.Errorf("expected LogLevel=debug after migration, got %q", got.LogLevel)
	}
	if !got.SensitiveLogging {
		t.Error("expected SensitiveLogging=true to be preserved")
	}
}

func TestMigration_LegacyDebugLoggingFalse_BecomesOff(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "KeyLint")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	legacy := `{"active_provider":"openai","debug_logging":false}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	svc := newServiceAt(t, tmp)
	got := svc.Get()
	if got.LogLevel != "off" {
		t.Errorf("expected LogLevel=off after migration, got %q", got.LogLevel)
	}
}

func TestMigration_NewLogLevel_LoadsCorrectly(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "KeyLint")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	newJSON := `{"active_provider":"openai","log_level":"warning"}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(newJSON), 0600); err != nil {
		t.Fatal(err)
	}
	svc := newServiceAt(t, tmp)
	got := svc.Get()
	if got.LogLevel != "warning" {
		t.Errorf("expected LogLevel=warning, got %q", got.LogLevel)
	}
}

func TestMigration_BothFields_LogLevelTakesPrecedence(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "KeyLint")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	// Both fields present — log_level wins
	both := `{"active_provider":"openai","debug_logging":true,"log_level":"error"}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(both), 0600); err != nil {
		t.Fatal(err)
	}
	svc := newServiceAt(t, tmp)
	got := svc.Get()
	if got.LogLevel != "error" {
		t.Errorf("expected LogLevel=error (explicit wins), got %q", got.LogLevel)
	}
}

func TestMigration_RoundTrip_LegacyToNew(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "KeyLint")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	legacy := `{"active_provider":"openai","debug_logging":true}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}

	// Load (migrates), then save, then reload
	svc := newServiceAt(t, tmp)
	if err := svc.Save(svc.Get()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read raw file — should have log_level, not debug_logging
	data, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	raw := string(data)
	if !strings.Contains(raw, `"log_level"`) {
		t.Error("saved file should contain log_level")
	}
	if strings.Contains(raw, `"debug_logging"`) {
		t.Error("saved file should not contain legacy debug_logging")
	}

	// Reload and verify
	svc2 := newServiceAt(t, tmp)
	got := svc2.Get()
	if got.LogLevel != "debug" {
		t.Errorf("round-trip: expected LogLevel=debug, got %q", got.LogLevel)
	}
}

func TestDefault_LogLevel_IsOff(t *testing.T) {
	d := settings.Default()
	if d.LogLevel != "off" {
		t.Errorf("expected default LogLevel=off, got %q", d.LogLevel)
	}
}
```

Note: you'll need to add `"strings"` to the import block.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/dev/projects/keylint && go test ./internal/features/settings/ -v -count=1 -run "Migration|Default_LogLevel"`
Expected: compilation errors (LogLevel field doesn't exist yet)

- [ ] **Step 3: Update the settings model**

In `internal/features/settings/model.go`, replace `DebugLogging bool` with `LogLevel string`:

Replace:
```go
	DebugLogging      bool     `json:"debug_logging"`      // writes debug.log to the app config dir
```
With:
```go
	LogLevel          string   `json:"log_level"`           // "off"|"trace"|"debug"|"info"|"warning"|"error"
```

Update `Default()` to set `LogLevel: "off"`:
```go
func Default() Settings {
	return Settings{
		ActiveProvider:             "openai",
		ShortcutKey:                "ctrl+g",
		ThemePreference:            "dark",
		LogLevel:                   "off",
		PyramidizeQualityThreshold: DefaultQualityThreshold,
	}
}
```

- [ ] **Step 4: Add migration logic to settings service**

In `internal/features/settings/service.go`, update the `load()` method. After the `json.Unmarshal` call, add migration logic:

```go
func (s *Service) load() error {
	data, err := os.ReadFile(s.filePath)
	if os.IsNotExist(err) {
		logger.Info("settings: file not found, using defaults", "path", s.filePath)
		return err
	}
	if err != nil {
		logger.Error("settings: read failed", "path", s.filePath, "err", err)
		return err
	}
	if err := json.Unmarshal(data, &s.current); err != nil {
		logger.Error("settings: unmarshal failed", "err", err)
		return err
	}

	// Migrate legacy debug_logging → log_level
	if s.current.LogLevel == "" {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err == nil {
			if dl, ok := raw["debug_logging"]; ok {
				var enabled bool
				if json.Unmarshal(dl, &enabled) == nil && enabled {
					s.current.LogLevel = "debug"
				} else {
					s.current.LogLevel = "off"
				}
			} else {
				s.current.LogLevel = "off"
			}
		}
	}

	logger.Info("settings: loaded", "path", s.filePath)
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/dev/projects/keylint && go test ./internal/features/settings/ -v -count=1`
Expected: all tests PASS (including new migration tests and existing tests)

- [ ] **Step 6: Commit**

```bash
git add internal/features/settings/model.go internal/features/settings/service.go internal/features/settings/service_test.go
git commit -m "feat(settings): migrate debug_logging bool to log_level string"
```

---

### Task 3: Update main.go and CLI — Wire Up New Logger Init

**Files:**
- Modify: `main.go:72`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/fix.go`
- Modify: `internal/cli/pyramidize.go`
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: Write the failing CLI tests for --log flag**

Append to `internal/cli/cli_test.go`:

```go
func TestLogFlagValidation_InvalidLevel(t *testing.T) {
	var stdout, stderr bytes.Buffer
	enhancer := &mockEnhancer{result: "fixed"}
	// Pass an invalid log level
	err := runFixWith([]string{"--log", "banana", "hello"}, &stdout, &stderr, enhancer)
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
	if !strings.Contains(err.Error(), "invalid log level") {
		t.Errorf("error should mention 'invalid log level', got: %v", err)
	}
}

func TestLogFlagValidation_ValidLevels(t *testing.T) {
	for _, level := range []string{"off", "trace", "debug", "info", "warning", "error"} {
		t.Run(level, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			enhancer := &mockEnhancer{result: "fixed"}
			err := runFixWith([]string{"--log", level, "hello"}, &stdout, &stderr, enhancer)
			if err != nil {
				t.Fatalf("unexpected error for level %q: %v", level, err)
			}
		})
	}
}

func TestLogFlagDefault_IsOff(t *testing.T) {
	// When --log is not provided, no error and command works
	var stdout, stderr bytes.Buffer
	enhancer := &mockEnhancer{result: "fixed"}
	err := runFixWith([]string{"hello"}, &stdout, &stderr, enhancer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPyramidizeLogFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mock := &mockPyramidizer{
		result: pyramidize.PyramidizeResult{
			FullDocument: "output",
			QualityFlags: []string{},
		},
	}
	err := runPyramidizeWith([]string{"--log", "debug", "some text"}, &stdout, &stderr, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

Note: add `"strings"` to the import block.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/dev/projects/keylint && go test ./internal/cli/ -v -count=1 -run "LogFlag"`
Expected: FAIL — `--log` flag not defined yet

- [ ] **Step 3: Add shared log flag helper to cli.go**

Add to `internal/cli/cli.go`:

```go
import (
	"fmt"
	"io"
	"os"
	"strings"

	"keylint/internal/features/enhance"
	"keylint/internal/features/settings"
	"keylint/internal/logger"
)

// validLogLevels lists accepted values for the --log flag.
var validLogLevels = map[string]bool{
	"off": true, "trace": true, "debug": true,
	"info": true, "warning": true, "error": true,
}

// addLogFlag registers a --log flag on the given FlagSet and returns the pointer.
func addLogFlag(fs *flag.FlagSet) *string {
	return fs.String("log", "off", "Log level: off|trace|debug|info|warning|error")
}

// initLogger validates the log level and initializes the logger.
// Sensitive is always false in CLI mode to prevent credential leaks to terminal.
func initLogger(level string) error {
	if !validLogLevels[level] {
		return fmt.Errorf("invalid log level %q — valid levels: off, trace, debug, info, warning, error", level)
	}
	logger.Init(level, false)
	return nil
}
```

Also add `"flag"` to the import block.

- [ ] **Step 4: Wire --log flag into runFixWith**

In `internal/cli/fix.go`, update `runFixWith` to accept and validate the flag:

```go
func runFixWith(args []string, stdout io.Writer, stderr io.Writer, svc enhancer) error {
	fs := flag.NewFlagSet("fix", flag.ContinueOnError)
	fs.SetOutput(stderr)
	filePath := fs.String("f", "", "Input file path")
	logLevel := addLogFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := initLogger(*logLevel); err != nil {
		return err
	}

	inlineText := strings.Join(fs.Args(), " ")
	text, err := readInput(*filePath, inlineText, stdinIfPiped())
	if err != nil {
		return err
	}

	result, err := svc.Enhance(text)
	if err != nil {
		return fmt.Errorf("enhance failed: %w", err)
	}

	fmt.Fprintln(stdout, result)
	return nil
}
```

- [ ] **Step 5: Wire --log flag into runPyramidizeWith**

In `internal/cli/pyramidize.go`, add the flag after the existing flags:

```go
func runPyramidizeWith(args []string, stdout io.Writer, stderr io.Writer, svc pyramidizer) error {
	fs := flag.NewFlagSet("pyramidize", flag.ContinueOnError)
	fs.SetOutput(stderr)

	filePath := fs.String("f", "", "Input file path")
	docType := fs.String("type", "auto", "Document type: auto|email|wiki|memo|powerpoint")
	jsonOut := fs.Bool("json", false, "Output full result as JSON")
	provider := fs.String("provider", "", "AI provider override: claude|openai|ollama")
	model := fs.String("model", "", "Model override (e.g. claude-sonnet-4-6)")
	style := fs.String("style", "professional", "Communication style")
	relationship := fs.String("relationship", "professional", "Relationship level")
	variant := fs.Int("variant", 0, "Prompt variant (0=latest, 1=v1, 2=v2)")
	logLevel := addLogFlag(fs)

	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := initLogger(*logLevel); err != nil {
		return err
	}

	inlineText := strings.Join(fs.Args(), " ")
	text, err := readInput(*filePath, inlineText, stdinIfPiped())
	if err != nil {
		return err
	}

	req := pyramidize.PyramidizeRequest{
		Text:               text,
		DocumentType:       *docType,
		CommunicationStyle: *style,
		RelationshipLevel:  *relationship,
		Provider:           *provider,
		Model:              *model,
		PromptVariant:      *variant,
	}

	result, err := svc.Pyramidize(req)
	if err != nil {
		return fmt.Errorf("pyramidize failed: %w", err)
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Fprintln(stdout, result.FullDocument)
	return nil
}
```

- [ ] **Step 6: Update main.go**

In `main.go:72`, change:
```go
	logger.Init(cfg.DebugLogging, cfg.SensitiveLogging)
```
To:
```go
	logger.Init(cfg.LogLevel, cfg.SensitiveLogging)
```

- [ ] **Step 7: Run all tests**

Run: `cd /home/dev/projects/keylint && go test ./internal/cli/ -v -count=1 && go test ./internal/logger/ -v -count=1 && go test ./internal/features/settings/ -v -count=1`
Expected: all PASS

- [ ] **Step 8: Verify build compiles**

Run: `cd /home/dev/projects/keylint && go build -o bin/KeyLint .`
Expected: compiles cleanly

- [ ] **Step 9: Commit**

```bash
git add main.go internal/cli/cli.go internal/cli/fix.go internal/cli/pyramidize.go internal/cli/cli_test.go
git commit -m "feat(cli): add --log flag for CLI commands, wire new logger.Init"
```

---

### Task 4: Sensitive Audit — Migrate All Sensitive() Calls to Redact()

**Files:**
- Modify: `internal/features/enhance/service.go`
- Modify: `internal/features/pyramidize/api_claude.go`
- Modify: `internal/features/pyramidize/api_openai.go`
- Modify: `internal/features/pyramidize/api_ollama.go`

- [ ] **Step 1: Migrate enhance/service.go**

Replace all `logger.Sensitive(...)` calls with `logger.Debug(...)` + `logger.Redact()`:

Line 107:
```go
// Before:
logger.Sensitive("enhance: request", "provider", "openai", "payload", string(payload))
// After:
logger.Debug("enhance: request", "provider", "openai", "payload", logger.Redact(string(payload)))
```

Line 118:
```go
// Before:
logger.Sensitive("enhance: response", "provider", "openai", "status", resp.StatusCode, "body", string(body))
// After:
logger.Debug("enhance: response", "provider", "openai", "status", resp.StatusCode, "body", logger.Redact(string(body)))
```

Line 140:
```go
// Before:
logger.Sensitive("enhance: request", "provider", "claude", "payload", string(payload))
// After:
logger.Debug("enhance: request", "provider", "claude", "payload", logger.Redact(string(payload)))
```

Line 152:
```go
// Before:
logger.Sensitive("enhance: response", "provider", "claude", "status", resp.StatusCode, "body", string(body))
// After:
logger.Debug("enhance: response", "provider", "claude", "status", resp.StatusCode, "body", logger.Redact(string(body)))
```

Line 174:
```go
// Before:
logger.Sensitive("enhance: request", "provider", "ollama", "payload", string(payload))
// After:
logger.Debug("enhance: request", "provider", "ollama", "payload", logger.Redact(string(payload)))
```

Line 184:
```go
// Before:
logger.Sensitive("enhance: response", "provider", "ollama", "status", resp.StatusCode, "body", string(body))
// After:
logger.Debug("enhance: response", "provider", "ollama", "status", resp.StatusCode, "body", logger.Redact(string(body)))
```

- [ ] **Step 2: Migrate pyramidize/api_claude.go**

Line 31:
```go
// Before:
logger.Sensitive("pyramidize: claude request", "len", len(payload))
// After:
logger.Debug("pyramidize: claude request", "payload", logger.Redact(string(payload)))
```

Line 48:
```go
// Before:
logger.Sensitive("pyramidize: claude response", "status", resp.StatusCode, "len", len(body))
// After:
logger.Debug("pyramidize: claude response", "status", resp.StatusCode, "body", logger.Redact(string(body)))
```

- [ ] **Step 3: Migrate pyramidize/api_openai.go**

Line 36:
```go
// Before:
logger.Sensitive("pyramidize: openai request", "len", len(payload))
// After:
logger.Debug("pyramidize: openai request", "payload", logger.Redact(string(payload)))
```

Line 52:
```go
// Before:
logger.Sensitive("pyramidize: openai response", "status", resp.StatusCode, "len", len(body))
// After:
logger.Debug("pyramidize: openai response", "status", resp.StatusCode, "body", logger.Redact(string(body)))
```

- [ ] **Step 4: Migrate pyramidize/api_ollama.go**

Line 37:
```go
// Before:
logger.Sensitive("pyramidize: ollama request", "len", len(payload))
// After:
logger.Debug("pyramidize: ollama request", "payload", logger.Redact(string(payload)))
```

Line 52:
```go
// Before:
logger.Sensitive("pyramidize: ollama response", "status", resp.StatusCode, "len", len(body))
// After:
logger.Debug("pyramidize: ollama response", "status", resp.StatusCode, "body", logger.Redact(string(body)))
```

- [ ] **Step 5: Remove the old Sensitive function from logger.go**

Delete these lines from `internal/logger/logger.go` (they should already be gone from the Task 1 rewrite, but verify):
```go
// Sensitive logs only when sensitive logging is enabled.
func Sensitive(msg string, args ...any) {
	if sensitiveEnabled {
		l.Debug(msg, args...)
	}
}
```

- [ ] **Step 6: Verify build compiles and all Go tests pass**

Run: `cd /home/dev/projects/keylint && go build -o bin/KeyLint . && go test ./internal/... -count=1`
Expected: compiles, all tests pass

- [ ] **Step 7: Commit**

```bash
git add internal/features/enhance/service.go internal/features/pyramidize/api_claude.go internal/features/pyramidize/api_openai.go internal/features/pyramidize/api_ollama.go internal/logger/logger.go
git commit -m "refactor(logging): migrate Sensitive() calls to Debug() + Redact()"
```

---

### Task 5: Frontend Log Bridge — Source Tagging and Redaction

**Files:**
- Modify: `internal/features/logger/service.go`
- Create: `internal/features/logger/service_test.go`

- [ ] **Step 1: Write the failing bridge tests**

Create `internal/features/logger/service_test.go`:

```go
package logger

import (
	"bytes"
	"strings"
	"testing"

	"keylint/internal/logger"
)

func TestLog_Error_AppearsWithSourceFrontend(t *testing.T) {
	var buf bytes.Buffer
	logger.InitWithWriter(&buf, "debug", false)
	svc := NewService()
	svc.Log("error", "something broke")
	output := buf.String()
	if !strings.Contains(output, "source=frontend") {
		t.Errorf("expected source=frontend, got:\n%s", output)
	}
	if !strings.Contains(output, "something broke") {
		t.Error("error message should always appear")
	}
}

func TestLog_Info_MsgRedactedWhenSensitiveOff(t *testing.T) {
	var buf bytes.Buffer
	logger.InitWithWriter(&buf, "debug", false) // sensitive OFF
	svc := NewService()
	svc.Log("info", "user typed secret text")
	output := buf.String()
	if strings.Contains(output, "user typed secret text") {
		t.Error("info msg should be redacted when sensitive is off")
	}
	if !strings.Contains(output, "[redacted]") {
		t.Error("expected [redacted] placeholder for msg")
	}
}

func TestLog_Info_MsgVisibleWhenSensitiveOn(t *testing.T) {
	var buf bytes.Buffer
	logger.InitWithWriter(&buf, "debug", true) // sensitive ON
	svc := NewService()
	svc.Log("info", "user typed secret text")
	output := buf.String()
	if !strings.Contains(output, "user typed secret text") {
		t.Error("info msg should be visible when sensitive is on")
	}
}

func TestLog_UnknownLevel_DefaultsToInfo(t *testing.T) {
	var buf bytes.Buffer
	logger.InitWithWriter(&buf, "debug", true)
	svc := NewService()
	svc.Log("banana", "unknown level msg")
	output := buf.String()
	if !strings.Contains(output, "unknown level msg") {
		t.Error("unknown level should default to info and appear at debug level")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/dev/projects/keylint && go test ./internal/features/logger/ -v -count=1`
Expected: FAIL — `InitWithWriter` not exported, service doesn't use FrontendLogger yet

- [ ] **Step 3: Export InitWithWriter from logger package**

In `internal/logger/logger.go`, rename `initWithWriter` to `InitWithWriter` (capitalize):

```go
// InitWithWriter is used by tests to redirect output to a buffer.
func InitWithWriter(w io.Writer, level string, sensitive bool) {
```

Also update `logger_test.go` — the `initWithBuffer` helper calls `initWithWriter`, change it to `InitWithWriter`.

- [ ] **Step 4: Rewrite the frontend bridge service**

Rewrite `internal/features/logger/service.go`:

```go
// Package logger exposes a Wails-registered service so the Angular frontend
// can forward log messages into the Go debug.log file.
package logger

import "keylint/internal/logger"

// Service forwards frontend log messages into the Go structured logger.
type Service struct{}

// NewService creates a new LogService.
func NewService() *Service { return &Service{} }

// Log writes a frontend message at the given level into debug.log.
// The msg is wrapped in Redact() because frontend messages may contain user text.
// Error and warn levels log the msg directly (operational), all others redact.
func (s *Service) Log(level, msg string) {
	fl := logger.FrontendLogger()
	switch level {
	case "trace":
		fl.Log(nil, logger.LevelTrace, "frontend", "msg", logger.Redact(msg))
	case "debug":
		fl.Debug("frontend", "msg", logger.Redact(msg))
	case "warn":
		fl.Warn("frontend", "msg", msg)
	case "error":
		fl.Error("frontend", "msg", msg)
	default:
		fl.Info("frontend", "msg", logger.Redact(msg))
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/dev/projects/keylint && go test ./internal/features/logger/ -v -count=1 && go test ./internal/logger/ -v -count=1`
Expected: all PASS

- [ ] **Step 6: Verify full Go test suite**

Run: `cd /home/dev/projects/keylint && go test ./internal/... -count=1`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add internal/logger/logger.go internal/logger/logger_test.go internal/features/logger/service.go internal/features/logger/service_test.go
git commit -m "feat(logger): frontend bridge with source tagging and Redact"
```

---

### Task 6: Settings UI — Replace Toggle with Select Dropdown

**Files:**
- Modify: `frontend/src/app/features/settings/settings.component.ts`
- Modify: `frontend/src/app/features/settings/settings.component.spec.ts`
- Modify: `frontend/src/testing/wails-mock.ts`
- Modify: `frontend/src/app/core/wails.service.ts`

- [ ] **Step 1: Write the failing UI tests**

Add these tests to `frontend/src/app/features/settings/settings.component.spec.ts`:

```typescript
  it('log level select renders with all 6 options', () => {
    const section = el.querySelector('[data-testid="log-level-section"]');
    expect(section).toBeTruthy();
    expect(section!.querySelector('p-select')).toBeTruthy();
  });

  it('sensitive toggle is disabled when log level is off', () => {
    component.settings!.log_level = 'off';
    fixture.detectChanges();
    const section = el.querySelector('[data-testid="sensitive-logging-section"]');
    const toggle = section!.querySelector('p-toggle-switch');
    expect(toggle).toBeTruthy();
  });

  it('save sends log_level to backend', async () => {
    component.settings!.log_level = 'warning';
    await component.save();
    expect(wailsMock.saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ log_level: 'warning' }),
    );
  });
```

- [ ] **Step 2: Update wails-mock.ts**

In `frontend/src/testing/wails-mock.ts`, update `defaultSettings`:

Replace `debug_logging: false` with `log_level: 'off'`:

```typescript
export const defaultSettings: Settings = {
  active_provider: 'openai',
  providers: {
    ollama_url: '',
    aws_region: '',
  },
  shortcut_key: 'ctrl+g',
  start_on_boot: false,
  theme_preference: 'dark',
  completed_setup: false,
  log_level: 'off',
  sensitive_logging: false,
  update_channel: '',
  app_presets: [],
  pyramidize_quality_threshold: 0.65,
};
```

- [ ] **Step 3: Update BROWSER_MODE_DEFAULTS in wails.service.ts**

In `frontend/src/app/core/wails.service.ts`, update the `BROWSER_MODE_DEFAULTS`:

Replace `debug_logging: false` with `log_level: 'off'`:

```typescript
const BROWSER_MODE_DEFAULTS: Settings = {
  active_provider: 'claude',
  providers: { ollama_url: '', aws_region: '' },
  shortcut_key: 'ctrl+g',
  start_on_boot: false,
  theme_preference: 'dark',
  completed_setup: false,
  log_level: 'off',
  sensitive_logging: false,
  update_channel: '',
  app_presets: [],
  pyramidize_quality_threshold: 0.65,
};
```

Note: The `Settings` type is auto-generated from Go bindings, so after the Go model change and `wails3 generate bindings`, the TS type will have `log_level: string` instead of `debug_logging: boolean`. If bindings can't be regenerated in the current environment, the type may need a manual interface update.

- [ ] **Step 4: Update the settings component template**

In `frontend/src/app/features/settings/settings.component.ts`, replace the debug-logging toggle section (lines 77-85) with a select dropdown:

Replace:
```html
                <div class="form-group" data-testid="debug-logging-section">
                  <div class="toggle-row">
                    <div class="toggle-label-group">
                      <label>Debug Logging</label>
                      <small class="hint-text">When enabled, writes a <code>debug.log</code> to the app config folder. Takes effect on next launch.</small>
                    </div>
                    <p-toggle-switch [(ngModel)]="settings.debug_logging" />
                  </div>
                </div>
```

With:
```html
                <div class="form-group" data-testid="log-level-section">
                  <label>Log Level</label>
                  <p-select
                    [(ngModel)]="settings.log_level"
                    [options]="logLevels"
                    optionLabel="label"
                    optionValue="value"
                  />
                  <small class="hint-text">Writes to <code>~/.config/KeyLint/debug.log</code> (Linux) or <code>%AppData%/KeyLint/debug.log</code> (Windows). Takes effect on next launch.</small>
                </div>
```

Update the sensitive logging toggle's disabled binding (line 92):

Replace:
```html
                    <p-toggle-switch [(ngModel)]="settings.sensitive_logging" [disabled]="!settings.debug_logging" />
```

With:
```html
                    <p-toggle-switch [(ngModel)]="settings.sensitive_logging" [disabled]="settings.log_level === 'off'" />
```

- [ ] **Step 5: Add logLevels options to the component class**

In the component class, add the `logLevels` array after `updateChannels`:

```typescript
  readonly logLevels = [
    { label: 'Off', value: 'off' },
    { label: 'Trace', value: 'trace' },
    { label: 'Debug', value: 'debug' },
    { label: 'Info', value: 'info' },
    { label: 'Warning', value: 'warning' },
    { label: 'Error', value: 'error' },
  ];
```

- [ ] **Step 6: Run frontend tests**

Run: `cd /home/dev/projects/keylint/frontend && npm test`
Expected: all tests PASS

- [ ] **Step 7: Update existing test assertions**

The test `debug-logging section contains both toggle and hint text` references `[data-testid="debug-logging-section"]` which no longer exists. Replace it:

```typescript
  it('log-level section contains select and hint text', () => {
    const section = el.querySelector('[data-testid="log-level-section"]');
    expect(section).toBeTruthy();
    expect(section!.querySelector('p-select')).toBeTruthy();
    expect(section!.querySelector('small')).toBeTruthy();
  });
```

- [ ] **Step 8: Run frontend tests again**

Run: `cd /home/dev/projects/keylint/frontend && npm test`
Expected: all tests PASS

- [ ] **Step 9: Commit**

```bash
git add frontend/src/app/features/settings/settings.component.ts frontend/src/app/features/settings/settings.component.spec.ts frontend/src/testing/wails-mock.ts frontend/src/app/core/wails.service.ts
git commit -m "feat(settings): replace debug toggle with log level dropdown"
```

---

### Task 7: Documentation — docs/logging.md and CLAUDE.md Reference

**Files:**
- Create: `docs/logging.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Create docs/logging.md**

```markdown
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

When `SensitiveLogging` is off, `Redact()` outputs `[redacted]`. When on, the real value is shown.

Never wrap: provider names, status codes, byte lengths, error messages, config keys.
Always wrap: API request/response bodies, user text, clipboard content, API keys.

## Source Tagging

All log entries include a `source` attribute:
- `source=backend` — Go backend (automatic via default logger)
- `source=frontend` — Angular frontend (via the log bridge service)

## CLI Usage

```bash
./bin/KeyLint -fix --log debug "text to fix"
./bin/KeyLint -pyramidize --log info -f input.md
```

Sensitive logging is always off in CLI mode.

## Settings

UI: Settings → General → Log Level dropdown (Off/Trace/Debug/Info/Warning/Error)
JSON field: `"log_level": "off"` (replaces legacy `"debug_logging": true/false`)
```

- [ ] **Step 2: Add reference to CLAUDE.md**

Add to the `CLAUDE.md` file, in the `## Rules & Reference` section, after the existing reference docs line:

```
**Logging conventions:** `docs/logging.md` (levels, Redact() usage, source tagging, CLI flags)
```

- [ ] **Step 3: Commit**

```bash
git add docs/logging.md CLAUDE.md
git commit -m "docs: add logging conventions reference"
```

---

### Task 8: Final Verification

- [ ] **Step 1: Run full Go test suite**

Run: `cd /home/dev/projects/keylint && go test ./internal/... -v -count=1`
Expected: all PASS

- [ ] **Step 2: Run frontend test suite**

Run: `cd /home/dev/projects/keylint/frontend && npm test`
Expected: all PASS

- [ ] **Step 3: Verify build compiles**

Run: `cd /home/dev/projects/keylint && go build -o bin/KeyLint .`
Expected: compiles cleanly

- [ ] **Step 4: Smoke test CLI logging**

Run: `cd /home/dev/projects/keylint && echo "test" | ./bin/KeyLint -fix --log debug 2>&1 || true`
Expected: no crash (API call may fail without key, but the logger should initialize without error)

- [ ] **Step 5: Verify no Sensitive() references remain**

Run: `grep -r "logger\.Sensitive\|\.Sensitive(" internal/ --include="*.go"`
Expected: no matches (all migrated to Redact)

- [ ] **Step 6: Final commit if any fixups needed**

Only if previous steps required changes.
