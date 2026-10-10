package settings_test

import (
	"os"
	"path/filepath"
	"testing"

	"keylint/internal/features/settings"
)

// NewService runs before logging is up, so main.go logs what it found from
// LoadOutcome instead. These pin what that line can say.

func TestLoadOutcome_NoFileMeansDefaults(t *testing.T) {
	svc := newServiceAt(t, t.TempDir())
	got := settings.LoadOutcome(svc)
	if got.Found {
		t.Error("Found = true with no settings file")
	}
	if got.CompletedSetup {
		t.Error("CompletedSetup = true from defaults")
	}
	if filepath.Base(got.Path) != "settings.json" {
		t.Errorf("Path = %q, want the settings file it looked for", got.Path)
	}
}

func TestLoadOutcome_ReportsTheFlagAsLoaded(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "KeyLint"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "KeyLint", "settings.json")
	if err := os.WriteFile(file, []byte(`{"completed_setup": true, "log_level": "debug"}`), 0600); err != nil {
		t.Fatal(err)
	}
	svc := newServiceAt(t, dir)

	// A later change must not rewrite what was loaded.
	if err := settings.Update(svc, func(c *settings.Settings) { c.CompletedSetup = false }); err != nil {
		t.Fatal(err)
	}

	got := settings.LoadOutcome(svc)
	if !got.Found || !got.CompletedSetup {
		t.Errorf("LoadOutcome = %+v, want found with completed_setup=true", got)
	}
}
