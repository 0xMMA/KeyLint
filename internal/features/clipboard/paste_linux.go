//go:build !windows

package clipboard

import (
	"os/exec"
	"time"

	"keylint/internal/logger"
)

// CopyFromForeground sends Ctrl+C to the currently focused window via xdotool,
// then waits 150 ms for the clipboard to be populated.
// Best-effort: if xdotool is not installed, logs a warning and returns nil.
func (s *Service) CopyFromForeground() error {
	if _, err := exec.LookPath("xdotool"); err != nil {
		logger.Warn("clipboard: CopyFromForeground skipped — xdotool not found in PATH")
		return nil
	}
	if err := exec.Command("xdotool", "key", "--clearmodifiers", "ctrl+c").Run(); err != nil {
		logger.Warn("clipboard: CopyFromForeground failed", "err", err)
	}
	time.Sleep(150 * time.Millisecond)
	return nil
}

// PasteToForeground sends a Ctrl+V keystroke to the currently focused window.
// Best-effort: if xdotool is not installed, logs a warning and returns nil.
func (s *Service) PasteToForeground() error {
	if _, err := exec.LookPath("xdotool"); err != nil {
		logger.Warn("clipboard: PasteToForeground skipped — xdotool not found in PATH")
		return nil
	}
	time.Sleep(150 * time.Millisecond)
	if err := exec.Command("xdotool", "key", "--clearmodifiers", "ctrl+v").Run(); err != nil {
		logger.Warn("clipboard: PasteToForeground failed", "err", err)
	}
	return nil
}
