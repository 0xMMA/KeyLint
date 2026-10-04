package settings

import (
	"os"
	"path/filepath"
	"time"
)

// writeFileAtomic replaces path with data without ever truncating the file in
// place.
//
// os.WriteFile truncates first, so a write that stops partway — a full disk,
// or the process dying mid-write — left a cut-off settings.json, and load()
// refuses to start the app on a file it cannot parse. Here the data goes to a
// sibling temp file first; the real file is only touched by the final rename,
// and if anything fails before that it is left exactly as it was.
//
// On Unix the rename is atomic. On Windows Go does not promise that (os.Rename
// uses MoveFileEx with MOVEFILE_REPLACE_EXISTING); what holds there is the
// part that matters here: the old file stays whole until a complete new one
// replaces it.
//
// write fills the temp file; it is a parameter so a test can fail it halfway.
func writeFileAtomic(path string, data []byte, write func(f *os.File, data []byte) error) (err error) {
	// Same directory, so the rename never crosses a filesystem.
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	closed := false
	defer func() {
		if err != nil {
			if !closed {
				_ = tmp.Close()
			}
			_ = os.Remove(tmpName)
		}
	}()

	// CreateTemp already uses 0600; set it explicitly so the file never depends
	// on that default. The keys are not in here, but log paths and presets are.
	if err = tmp.Chmod(0o600); err != nil {
		return err
	}
	if err = write(tmp, data); err != nil {
		return err
	}
	// On disk before the rename, or a crash right after it could leave an
	// empty file under the real name.
	if err = tmp.Sync(); err != nil {
		return err
	}
	closed = true
	if err = tmp.Close(); err != nil {
		return err
	}
	return renameWithRetry(tmpName, path)
}

// writeAll is the production writer for writeFileAtomic.
func writeAll(f *os.File, data []byte) error {
	_, err := f.Write(data)
	return err
}

// Swappable for tests. renameRetryBudget is roughly what Go's own toolchain
// allows for the same problem (cmd/internal/robustio, golang/go#31247).
var (
	renameFile        = os.Rename
	sleep             = time.Sleep
	renameRetryBudget = 2 * time.Second
)

// renameWithRetry renames, retrying for a bounded time while the error is one
// that clears by itself. On Windows a virus scanner, the search indexer, or a
// `KeyLint -fix` run reading settings.json can hold the file open for a moment,
// and the rename fails with access denied or a sharing violation until it lets
// go. Elsewhere isTransientRenameError is always false, so this is one rename.
func renameWithRetry(from, to string) error {
	var slept time.Duration
	delay := 10 * time.Millisecond
	for {
		err := renameFile(from, to)
		if err == nil || !isTransientRenameError(err) || slept >= renameRetryBudget {
			return err
		}
		sleep(delay)
		slept += delay
		delay = min(delay*2, 500*time.Millisecond)
	}
}
