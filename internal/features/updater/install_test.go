package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minio/selfupdate"
)

// TestSelfupdateFailsOnReadOnlyDirectory documents the root cause of GH #18:
// selfupdate.Apply() cannot write to directories that require elevated permissions
// (e.g. C:\Program Files\ on Windows). This test ensures we never regress to the
// old in-place replacement approach for directories that aren't user-writable.
func TestSelfupdateFailsOnReadOnlyDirectory(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "KeyLint.exe")
	if err := os.WriteFile(fakeBin, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Make the directory read-only so selfupdate cannot create .KeyLint.exe.new
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	err := selfupdate.Apply(strings.NewReader("new binary"), selfupdate.Options{
		TargetPath: fakeBin,
	})

	if err == nil {
		t.Fatal("expected permission error when applying update to read-only directory, got nil — " +
			"this means the old selfupdate.Apply() approach would silently corrupt the install")
	}

	// The error message varies by OS: "permission denied" (Linux) or "Access is denied" (Windows).
	errMsg := strings.ToLower(err.Error())
	if !strings.Contains(errMsg, "permission denied") && !strings.Contains(errMsg, "access is denied") {
		t.Errorf("expected permission-related error, got: %v", err)
	}
}
