//go:build !windows

package updater

import (
	"fmt"
	"os"

	"github.com/minio/selfupdate"
)

// applyPlatformUpdate applies the downloaded binary in-place using selfupdate.
// On Linux the release asset is a raw executable, so direct replacement works
// as long as the target directory is writable.
func applyPlatformUpdate(svc *Service, tmpPath string) (InstallResult, error) {
	f, err := os.Open(tmpPath)
	if err != nil {
		return InstallResult{}, fmt.Errorf("opening downloaded update: %w", err)
	}
	defer f.Close()

	if err := selfupdate.Apply(f, selfupdate.Options{}); err != nil {
		return InstallResult{}, fmt.Errorf("applying update: %w — if this is a permission error, "+
			"the downloaded file is at %s — install manually or run with appropriate permissions",
			err, tmpPath)
	}

	os.Remove(tmpPath)
	return InstallResult{RestartRequired: false}, nil
}
