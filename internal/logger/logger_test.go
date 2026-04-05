package logger

import (
	"bytes"
	"io"
	"log/slog"
	"testing"
)

// initWithBuffer is a test helper that calls initWithWriter with a bytes.Buffer
// and returns the buffer. It registers cleanup to reset package-level state.
func initWithBuffer(t *testing.T, level string, sensitiveFlag bool) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	InitWithWriter(&buf, level, sensitiveFlag)
	t.Cleanup(func() {
		l = slog.New(slog.NewTextHandler(io.Discard, nil))
		sensitive.Store(false)
	})
	return &buf
}

// --- Level filtering tests ---

func TestLevelOff_NoOutput(t *testing.T) {
	buf := initWithBuffer(t, "off", false)
	Trace("trace msg")
	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")
	if buf.Len() != 0 {
		t.Errorf("expected no output for level=off, got: %s", buf.String())
	}
}

func TestLevelError_OnlyError(t *testing.T) {
	buf := initWithBuffer(t, "error", false)
	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")
	out := buf.String()
	if !containsSubstring(out, "error msg") {
		t.Error("expected error msg in output")
	}
	if containsSubstring(out, "debug msg") || containsSubstring(out, "info msg") || containsSubstring(out, "warn msg") {
		t.Errorf("expected only error output, got: %s", out)
	}
}

func TestLevelWarning_WarnAndError(t *testing.T) {
	buf := initWithBuffer(t, "warning", false)
	Trace("trace msg")
	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")
	out := buf.String()
	if !containsSubstring(out, "warn msg") {
		t.Error("expected warn msg in output")
	}
	if !containsSubstring(out, "error msg") {
		t.Error("expected error msg in output")
	}
	if containsSubstring(out, "trace msg") || containsSubstring(out, "debug msg") || containsSubstring(out, "info msg") {
		t.Errorf("expected only warn+error, got: %s", out)
	}
}

func TestLevelInfo_InfoWarnError(t *testing.T) {
	buf := initWithBuffer(t, "info", false)
	Trace("trace msg")
	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")
	out := buf.String()
	if !containsSubstring(out, "info msg") {
		t.Error("expected info msg in output")
	}
	if !containsSubstring(out, "warn msg") {
		t.Error("expected warn msg in output")
	}
	if !containsSubstring(out, "error msg") {
		t.Error("expected error msg in output")
	}
	if containsSubstring(out, "trace msg") || containsSubstring(out, "debug msg") {
		t.Errorf("expected no trace/debug, got: %s", out)
	}
}

func TestLevelDebug_DebugInfoWarnError(t *testing.T) {
	buf := initWithBuffer(t, "debug", false)
	Trace("trace msg")
	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")
	out := buf.String()
	if !containsSubstring(out, "debug msg") {
		t.Error("expected debug msg in output")
	}
	if !containsSubstring(out, "info msg") {
		t.Error("expected info msg in output")
	}
	if !containsSubstring(out, "warn msg") {
		t.Error("expected warn msg in output")
	}
	if !containsSubstring(out, "error msg") {
		t.Error("expected error msg in output")
	}
	if containsSubstring(out, "trace msg") {
		t.Errorf("expected no trace, got: %s", out)
	}
}

func TestLevelTrace_AllLevels(t *testing.T) {
	buf := initWithBuffer(t, "trace", false)
	Trace("trace msg")
	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")
	out := buf.String()
	for _, msg := range []string{"trace msg", "debug msg", "info msg", "warn msg", "error msg"} {
		if !containsSubstring(out, msg) {
			t.Errorf("expected %q in output, got: %s", msg, out)
		}
	}
}

func TestLevelInvalid_FallsBackToOff(t *testing.T) {
	buf := initWithBuffer(t, "bogus", false)
	Trace("trace msg")
	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")
	if buf.Len() != 0 {
		t.Errorf("expected no output for invalid level, got: %s", buf.String())
	}
}

// --- Redact tests ---

func TestRedact_SensitiveFalse_ShowsRedacted(t *testing.T) {
	buf := initWithBuffer(t, "info", false)
	Info("secret", "key", Redact("my-api-key-12345"))
	out := buf.String()
	if !containsSubstring(out, "[redacted]") {
		t.Errorf("expected [redacted] in output, got: %s", out)
	}
	if containsSubstring(out, "my-api-key-12345") {
		t.Errorf("expected secret NOT in output, got: %s", out)
	}
}

func TestRedact_SensitiveTrue_ShowsValue(t *testing.T) {
	buf := initWithBuffer(t, "info", true)
	Info("secret", "key", Redact("my-api-key-12345"))
	out := buf.String()
	if !containsSubstring(out, "my-api-key-12345") {
		t.Errorf("expected actual value in output, got: %s", out)
	}
}

func TestRedact_Nil_SensitiveFalse_ShowsRedacted(t *testing.T) {
	buf := initWithBuffer(t, "info", false)
	Info("secret", "key", Redact(nil))
	out := buf.String()
	if !containsSubstring(out, "[redacted]") {
		t.Errorf("expected [redacted] in output, got: %s", out)
	}
}

func TestRedact_ByteSlice_SensitiveTrue_ShowsValue(t *testing.T) {
	buf := initWithBuffer(t, "info", true)
	Info("payload", "body", Redact([]byte("request-body-content")))
	out := buf.String()
	if !containsSubstring(out, "request-body-content") {
		t.Errorf("expected byte slice value in output, got: %s", out)
	}
}

func TestRedact_Negative_SecretNeverInOutput(t *testing.T) {
	buf := initWithBuffer(t, "trace", false)
	secret := "super-secret-token-xyz789"
	Trace("auth", "token", Redact(secret))
	Debug("auth", "token", Redact(secret))
	Info("auth", "token", Redact(secret))
	Warn("auth", "token", Redact(secret))
	Error("auth", "token", Redact(secret))
	out := buf.String()
	if containsSubstring(out, secret) {
		t.Errorf("secret %q must NOT appear in output, got: %s", secret, out)
	}
}

// --- Source tagging tests ---

func TestBackendLogger_SourceTagBackend(t *testing.T) {
	buf := initWithBuffer(t, "info", false)
	Info("hello from backend")
	out := buf.String()
	if !containsSubstring(out, "source=backend") {
		t.Errorf("expected source=backend in output, got: %s", out)
	}
}

func TestFrontendLogger_SourceTagFrontend(t *testing.T) {
	buf := initWithBuffer(t, "info", false)
	fl := FrontendLogger()
	fl.Info("hello from frontend")
	out := buf.String()
	if !containsSubstring(out, "source=frontend") {
		t.Errorf("expected source=frontend in output, got: %s", out)
	}
	if containsSubstring(out, "source=backend") {
		t.Errorf("expected NO source=backend in frontend logger output, got: %s", out)
	}
}

// containsSubstring is a small helper for readable assertions.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && bytes.Contains([]byte(s), []byte(substr))
}
