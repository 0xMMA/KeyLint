package settings

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// An interrupted write must leave the previous file whole: load() refuses to
// start the app on a settings.json it cannot parse.
func TestWriteFileAtomic_KeepsTheOldFileWhenTheWriteIsCutOff(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	old := []byte(`{"active_provider":"openai"}`)
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatal(err)
	}

	cutOff := errors.New("disk full")
	err := writeFileAtomic(path, []byte(`{"active_provider":"claude-code"}`), func(f *os.File, data []byte) error {
		_, _ = f.Write(data[:len(data)/2])
		return cutOff
	})

	if !errors.Is(err, cutOff) {
		t.Fatalf("err = %v, want the writer's error", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(old) {
		t.Errorf("file = %q, want the old content %q untouched", got, old)
	}
	assertOnlyFile(t, dir, "settings.json")
}

func TestWriteFileAtomic_ReplacesTheFileAndLeavesNoTempBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"old":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	want := []byte(`{"new":true}`)
	if err := writeFileAtomic(path, want, writeAll); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("file = %q, want %q", got, want)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("mode = %o, want 600", perm)
		}
	}
	assertOnlyFile(t, dir, "settings.json")
}

func assertOnlyFile(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %v, want only %s", names, name)
	}
}
