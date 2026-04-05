//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
)

// applyPlatformUpdate launches the downloaded NSIS installer and signals the app to quit.
// The NSIS installer's own manifest requests UAC elevation, so no special privileges
// are needed here — Windows will show the UAC prompt to the user.
func applyPlatformUpdate(svc *Service, tmpPath string) (InstallResult, error) {
	cmd := exec.Command(tmpPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		os.Remove(tmpPath)
		return InstallResult{}, fmt.Errorf("launching installer: %w", err)
	}

	// Schedule app quit so the installer can replace the locked executable.
	// The quitFunc is set by main.go and includes a short delay for the
	// frontend to display the "closing" message before the app exits.
	if svc.quitFunc != nil {
		go svc.quitFunc()
	}

	return InstallResult{RestartRequired: true}, nil
}
