//go:build !windows

package pyramidize

import (
	"os/exec"
	"strings"
)

// captureSourceApp returns the name and xdotool window ID of the currently
// focused window. Returns empty strings if xdotool is not available or fails.
func captureSourceApp() (name string, windowID string) {
	// Get the active window ID
	idOut, err := exec.Command("xdotool", "getactivewindow").Output()
	if err != nil {
		return "", ""
	}
	id := strings.TrimSpace(string(idOut))
	if id == "" {
		return "", ""
	}

	// Get the window name/title
	nameOut, err := exec.Command("xdotool", "getwindowname", id).Output()
	if err != nil {
		return "", ""
	}
	return strings.TrimSpace(string(nameOut)), id
}

// sendBackToWindow focuses the given xdotool window and sends Ctrl+V to paste.
// Best-effort: returns nil if windowID is empty or xdotool is not available.
func sendBackToWindow(windowID string) error {
	if windowID == "" {
		return nil
	}
	if _, err := exec.LookPath("xdotool"); err != nil {
		return nil // xdotool not installed — silently skip
	}
	if err := exec.Command("xdotool", "windowfocus", windowID).Run(); err != nil {
		return nil // best-effort — focus may fail if window closed
	}
	_ = exec.Command("xdotool", "key", "--window", windowID, "ctrl+v").Run()
	return nil
}
