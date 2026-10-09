package settings_test

import (
	"strings"
	"testing"

	"keylint/internal/features/settings"
)

// Values the hook cannot use, found in a file, are put back to working ones
// on load, so the screen shows what the hook does and a hand-edited or
// damaged file cannot switch every shortcut off.
func TestLoad_NormalisesUnusableShortcutValues(t *testing.T) {
	tmp := t.TempDir()
	writeLegacySettings(t, tmp, `{"shortcut_mode": "", "shortcut_fix": "strg+g", "shortcut_pyramidize": "g", "shortcut_double_tap_delay": 1000}`)

	got := newServiceAt(t, tmp).Get()

	if got.ShortcutMode != "double_tap" {
		t.Errorf("mode: got %q, want double_tap", got.ShortcutMode)
	}
	if got.ShortcutFix != "ctrl+g" {
		t.Errorf("fix: got %q, want ctrl+g", got.ShortcutFix)
	}
	if got.ShortcutPyramidize != "ctrl+shift+g" {
		t.Errorf("pyramidize: got %q, want ctrl+shift+g", got.ShortcutPyramidize)
	}
	if got.ShortcutDoubleTapDelay != 500 {
		t.Errorf("delay: got %d, want 500 (clamped)", got.ShortcutDoubleTapDelay)
	}
}

func TestLoad_KeepsUsableShortcutValues(t *testing.T) {
	tmp := t.TempDir()
	writeLegacySettings(t, tmp, `{"shortcut_mode": "independent", "shortcut_fix": "alt+k", "shortcut_pyramidize": "f8", "shortcut_double_tap_delay": 150}`)

	got := newServiceAt(t, tmp).Get()

	if got.ShortcutMode != "independent" || got.ShortcutFix != "alt+k" ||
		got.ShortcutPyramidize != "f8" || got.ShortcutDoubleTapDelay != 150 {
		t.Errorf("usable values changed on load: %+v", got)
	}
}

// A save that changes the shortcuts must leave them working: the settings
// screen shows the error instead of writing a combo the hook would reject.
func TestSave_RejectsAShortcutTheHookCannotUse(t *testing.T) {
	cases := map[string]func(s *settings.Settings){
		"unknown key":        func(s *settings.Settings) { s.ShortcutFix = "ctrl+!" },
		"no modifier":        func(s *settings.Settings) { s.ShortcutFix = "g" },
		"bad pyramidize":     func(s *settings.Settings) { s.ShortcutMode = "independent"; s.ShortcutPyramidize = "shift+g" },
		"same combo twice":   func(s *settings.Settings) { s.ShortcutMode = "independent"; s.ShortcutPyramidize = s.ShortcutFix },
		"unknown mode":       func(s *settings.Settings) { s.ShortcutMode = "triple_tap" },
		"delay out of range": func(s *settings.Settings) { s.ShortcutDoubleTapDelay = 50 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			svc := newServiceAt(t, t.TempDir())
			before := svc.Get()
			updated := svc.Get()
			mutate(&updated)
			err := svc.Save(updated)
			if err == nil {
				t.Fatal("Save accepted it")
			}
			if !strings.Contains(err.Error(), "shortcut") {
				t.Errorf("error should name the shortcut: %v", err)
			}
			if got := svc.Get(); got.ShortcutFix != before.ShortcutFix || got.ShortcutMode != before.ShortcutMode {
				t.Errorf("settings changed despite the error: %+v", got)
			}
		})
	}
}

// A bad Pyramidize combo is irrelevant in double-tap mode, where the hook
// never reads it; it must not block saving something else.
func TestSave_IgnoresPyramidizeComboInDoubleTapMode(t *testing.T) {
	svc := newServiceAt(t, t.TempDir())
	updated := svc.Get()
	updated.ShortcutMode = "double_tap"
	updated.ShortcutPyramidize = ""
	updated.ShortcutFix = "ctrl+alt+k"
	if err := svc.Save(updated); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

// main.go re-applies the shortcuts from this callback, so it has to see every
// write — Save and Update alike — and never one that failed.
func TestOnSaved_SeesEverySuccessfulSave(t *testing.T) {
	svc := newServiceAt(t, t.TempDir())
	var calls []string
	settings.OnSaved(svc, func(old, updated settings.Settings) {
		calls = append(calls, old.ShortcutFix+">"+updated.ShortcutFix+"/"+updated.ActiveProvider)
	})

	updated := svc.Get()
	updated.ShortcutFix = "ctrl+alt+k"
	if err := svc.Save(updated); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := svc.SetActiveProvider("ollama"); err != nil {
		t.Fatalf("SetActiveProvider: %v", err)
	}
	bad := svc.Get()
	bad.ShortcutFix = "g"
	_ = svc.Save(bad)

	want := []string{"ctrl+g>ctrl+alt+k/openai", "ctrl+alt+k>ctrl+alt+k/ollama"}
	if strings.Join(calls, " ") != strings.Join(want, " ") {
		t.Errorf("calls: got %v, want %v", calls, want)
	}
}
