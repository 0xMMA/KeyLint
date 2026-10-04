package settings

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
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

// swapRename replaces the rename, the transient-error check and the sleep for
// one test, recording each sleep instead of waiting.
func swapRename(t *testing.T, rename func(string, string) error, transient func(error) bool) *[]time.Duration {
	t.Helper()
	oldRename, oldTransient, oldSleep := renameFile, isTransientRenameError, sleep
	t.Cleanup(func() { renameFile, isTransientRenameError, sleep = oldRename, oldTransient, oldSleep })
	var slept []time.Duration
	renameFile, isTransientRenameError = rename, transient
	sleep = func(d time.Duration) { slept = append(slept, d) }
	return &slept
}

var errHeldOpen = errors.New("the file is in use by another process")

// A scanner or indexer holding settings.json for a moment must not fail the
// save: the rename is retried until the handle is gone.
func TestWriteFileAtomic_RetriesARenameBlockedForAMoment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	failures := 3
	slept := swapRename(t, func(from, to string) error {
		if failures > 0 {
			failures--
			return errHeldOpen
		}
		return os.Rename(from, to)
	}, func(err error) bool { return errors.Is(err, errHeldOpen) })

	if err := writeFileAtomic(path, []byte(`{}`), writeAll); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	if len(*slept) != 3 {
		t.Errorf("slept %d times, want 3 (one per blocked rename)", len(*slept))
	}
	assertOnlyFile(t, dir, "settings.json")
}

// The retry is bounded: a file held open for good fails the save, within
// roughly the budget, and leaves no temp file behind.
func TestWriteFileAtomic_GivesUpOnARenameThatStaysBlocked(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	slept := swapRename(t, func(string, string) error { return errHeldOpen },
		func(err error) bool { return errors.Is(err, errHeldOpen) })

	err := writeFileAtomic(path, []byte(`{}`), writeAll)

	if !errors.Is(err, errHeldOpen) {
		t.Fatalf("err = %v, want the rename's error", err)
	}
	var total time.Duration
	for _, d := range *slept {
		total += d
	}
	if total < renameRetryBudget || total > renameRetryBudget+time.Second {
		t.Errorf("waited %v in total, want about %v", total, renameRetryBudget)
	}
	assertNoFiles(t, dir)
}

// Any other rename error fails at once, and the temp file goes with it.
func TestWriteFileAtomic_FailedRenameLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	permanent := errors.New("no such device")
	slept := swapRename(t, func(string, string) error { return permanent },
		func(err error) bool { return errors.Is(err, errHeldOpen) })

	err := writeFileAtomic(path, []byte(`{}`), writeAll)

	if !errors.Is(err, permanent) {
		t.Fatalf("err = %v, want the rename's error", err)
	}
	if len(*slept) != 0 {
		t.Errorf("slept %d times on a permanent error, want 0", len(*slept))
	}
	assertNoFiles(t, dir)
}

func assertNoFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("left behind: %s", e.Name())
	}
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
