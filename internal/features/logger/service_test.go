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
	t.Cleanup(func() { logger.InitWithWriter(&bytes.Buffer{}, "off", false) })

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
	logger.InitWithWriter(&buf, "debug", false)
	t.Cleanup(func() { logger.InitWithWriter(&bytes.Buffer{}, "off", false) })

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
	logger.InitWithWriter(&buf, "debug", true)
	t.Cleanup(func() { logger.InitWithWriter(&bytes.Buffer{}, "off", false) })

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
	t.Cleanup(func() { logger.InitWithWriter(&bytes.Buffer{}, "off", false) })

	svc := NewService()
	svc.Log("banana", "unknown level msg")
	output := buf.String()
	if !strings.Contains(output, "unknown level msg") {
		t.Error("unknown level should default to info and appear")
	}
}

func TestLog_Warn_MsgNotRedacted(t *testing.T) {
	var buf bytes.Buffer
	logger.InitWithWriter(&buf, "debug", false) // sensitive OFF
	t.Cleanup(func() { logger.InitWithWriter(&bytes.Buffer{}, "off", false) })

	svc := NewService()
	svc.Log("warn", "low disk space")
	output := buf.String()
	if !strings.Contains(output, "low disk space") {
		t.Error("warn msg should appear unredacted")
	}
	if !strings.Contains(output, "source=frontend") {
		t.Error("expected source=frontend")
	}
}
