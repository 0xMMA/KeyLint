package settings

import (
	"os"
	"path/filepath"
)

// writeFileAtomic replaces path with data so that a reader sees either the old
// file or the new one, never a mix.
//
// os.WriteFile truncates first. An interrupted write (a full disk, an
// antivirus lock on Windows, a crash) would leave a cut-off settings.json, and
// load() refuses to start the app on a file it cannot parse. Writing a sibling
// temp file and renaming it over the original avoids that: the rename is a
// single replace on every platform Go supports, Windows included.
//
// write fills the temp file; it is a parameter so a test can fail it halfway.
func writeFileAtomic(path string, data []byte, write func(f *os.File, data []byte) error) (err error) {
	// Same directory, so the rename never crosses a filesystem.
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	// CreateTemp already uses 0600; set it explicitly so the file never depends
	// on that default. The keys are not in here, but logs paths and presets are.
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
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// writeAll is the production writer for writeFileAtomic.
func writeAll(f *os.File, data []byte) error {
	_, err := f.Write(data)
	return err
}
