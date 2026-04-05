package settings_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"keylint/internal/features/settings"
)

// newServiceAt creates a Service pointed at a specific directory — used in tests
// to avoid writing to the real user config directory.
func newServiceAt(t *testing.T, dir string) *settings.Service {
	t.Helper()
	// os.UserConfigDir() reads XDG_CONFIG_HOME on Linux/macOS and APPDATA on Windows.
	var envKey string
	if runtime.GOOS == "windows" {
		envKey = "APPDATA"
	} else {
		envKey = "XDG_CONFIG_HOME"
	}
	original := os.Getenv(envKey)
	t.Cleanup(func() { os.Setenv(envKey, original) })
	os.Setenv(envKey, dir)

	svc, err := settings.NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestDefault_HasSensibleValues(t *testing.T) {
	d := settings.Default()
	if d.ActiveProvider != "openai" {
		t.Errorf("expected active_provider=openai, got %q", d.ActiveProvider)
	}
	if d.ShortcutKey != "ctrl+g" {
		t.Errorf("expected shortcut_key=ctrl+g, got %q", d.ShortcutKey)
	}
	if d.ThemePreference != "dark" {
		t.Errorf("expected theme_preference=dark, got %q", d.ThemePreference)
	}
	if d.CompletedSetup {
		t.Error("expected completed_setup=false by default")
	}
}

func TestNewService_ReturnsDefaults_WhenNoFileExists(t *testing.T) {
	tmp := t.TempDir()
	svc := newServiceAt(t, tmp)

	got := svc.Get()
	want := settings.Default()

	if got.ActiveProvider != want.ActiveProvider {
		t.Errorf("active_provider: got %q, want %q", got.ActiveProvider, want.ActiveProvider)
	}
	if got.ShortcutKey != want.ShortcutKey {
		t.Errorf("shortcut_key: got %q, want %q", got.ShortcutKey, want.ShortcutKey)
	}
}

func TestSave_PersistsToDisk(t *testing.T) {
	tmp := t.TempDir()
	svc := newServiceAt(t, tmp)

	updated := settings.Default()
	updated.ActiveProvider = "claude"
	updated.CompletedSetup = true

	if err := svc.Save(updated); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got := svc.Get()
	if got.ActiveProvider != "claude" {
		t.Errorf("after Save: active_provider=%q, want claude", got.ActiveProvider)
	}
	if !got.CompletedSetup {
		t.Error("after Save: expected completed_setup=true")
	}
}

func TestSave_WritesValidJSON(t *testing.T) {
	tmp := t.TempDir()
	svc := newServiceAt(t, tmp)

	updated := settings.Default()
	updated.ActiveProvider = "ollama"
	if err := svc.Save(updated); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Find and parse the written file.
	filePath := filepath.Join(tmp, "KeyLint", "settings.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var parsed settings.Settings
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if parsed.ActiveProvider != "ollama" {
		t.Errorf("parsed active_provider=%q, want ollama", parsed.ActiveProvider)
	}
}

func TestNewService_LoadsExistingFile(t *testing.T) {
	tmp := t.TempDir()

	// Write a settings file before creating the service.
	dir := filepath.Join(tmp, "KeyLint")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	existing := settings.Settings{
		ActiveProvider:  "bedrock",
		ShortcutKey:     "ctrl+shift+f",
		ThemePreference: "dark",
		CompletedSetup:  true,
	}
	data, _ := json.Marshal(existing)
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), data, 0600); err != nil {
		t.Fatal(err)
	}

	svc := newServiceAt(t, tmp)
	got := svc.Get()

	if got.ActiveProvider != "bedrock" {
		t.Errorf("active_provider: got %q, want bedrock", got.ActiveProvider)
	}
	if got.ThemePreference != "dark" {
		t.Errorf("theme_preference: got %q, want dark", got.ThemePreference)
	}
	if !got.CompletedSetup {
		t.Error("expected completed_setup=true from loaded file")
	}
}

// --- LogLevel migration tests ---

// writeLegacySettings writes raw JSON to the settings file for a temp-dir-based service.
func writeLegacySettings(t *testing.T, dir string, rawJSON string) {
	t.Helper()
	settingsDir := filepath.Join(dir, "KeyLint")
	if err := os.MkdirAll(settingsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(rawJSON), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestMigration_LegacyDebugLoggingTrue_SetsLogLevelDebug(t *testing.T) {
	tmp := t.TempDir()
	writeLegacySettings(t, tmp, `{"debug_logging": true}`)

	svc := newServiceAt(t, tmp)
	got := svc.Get()

	if got.LogLevel != "debug" {
		t.Errorf("expected LogLevel=debug for legacy debug_logging=true, got %q", got.LogLevel)
	}
}

func TestMigration_LegacyDebugLoggingFalse_SetsLogLevelOff(t *testing.T) {
	tmp := t.TempDir()
	writeLegacySettings(t, tmp, `{"debug_logging": false}`)

	svc := newServiceAt(t, tmp)
	got := svc.Get()

	if got.LogLevel != "off" {
		t.Errorf("expected LogLevel=off for legacy debug_logging=false, got %q", got.LogLevel)
	}
}

func TestMigration_NewLogLevel_LoadsCorrectly(t *testing.T) {
	tmp := t.TempDir()
	writeLegacySettings(t, tmp, `{"log_level": "warning"}`)

	svc := newServiceAt(t, tmp)
	got := svc.Get()

	if got.LogLevel != "warning" {
		t.Errorf("expected LogLevel=warning, got %q", got.LogLevel)
	}
}

func TestMigration_BothFields_LogLevelTakesPrecedence(t *testing.T) {
	tmp := t.TempDir()
	writeLegacySettings(t, tmp, `{"debug_logging": true, "log_level": "warning"}`)

	svc := newServiceAt(t, tmp)
	got := svc.Get()

	if got.LogLevel != "warning" {
		t.Errorf("expected LogLevel=warning (log_level takes precedence), got %q", got.LogLevel)
	}
}

func TestMigration_RoundTrip_LegacyToNewFormat(t *testing.T) {
	tmp := t.TempDir()
	writeLegacySettings(t, tmp, `{"debug_logging": true, "active_provider": "openai"}`)

	svc := newServiceAt(t, tmp)
	got := svc.Get()

	// Save the migrated settings.
	if err := svc.Save(got); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read the raw file and check the JSON.
	filePath := filepath.Join(tmp, "KeyLint", "settings.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	raw := string(data)

	if !strings.Contains(raw, `"log_level"`) {
		t.Error("saved file should contain log_level field")
	}
	if strings.Contains(raw, `"debug_logging"`) {
		t.Error("saved file should NOT contain legacy debug_logging field")
	}

	// Reload and verify.
	svc2 := newServiceAt(t, tmp)
	got2 := svc2.Get()
	if got2.LogLevel != "debug" {
		t.Errorf("after round-trip, expected LogLevel=debug, got %q", got2.LogLevel)
	}
}

func TestDefault_LogLevel_IsOff(t *testing.T) {
	d := settings.Default()
	if d.LogLevel != "off" {
		t.Errorf("expected Default().LogLevel=off, got %q", d.LogLevel)
	}
}
