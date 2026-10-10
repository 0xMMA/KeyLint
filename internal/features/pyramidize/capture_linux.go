//go:build !windows

package pyramidize

import (
	"os/exec"
	"strconv"
	"strings"
)

// The development desktop (Linux, X11) goes through xdotool, best-effort like
// the clipboard service: without it the source window is unknown and Send
// Back leaves the text on the clipboard.

// windowTitle is the window's title, "" if xdotool cannot say.
func windowTitle(id uintptr) string {
	out, err := exec.Command("xdotool", "getwindowname", strconv.FormatUint(uint64(id), 10)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// isMinimised is always false here: xdotool cannot ask, and windowactivate
// maps a minimised window anyway.
func isMinimised(uintptr) bool { return false }

// activateWindow asks the window manager to raise and focus the window.
func activateWindow(id uintptr) {
	_ = exec.Command("xdotool", "windowactivate", strconv.FormatUint(uint64(id), 10)).Run()
}
