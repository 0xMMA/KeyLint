//go:build !windows

package silentfix

import (
	"os/exec"
	"strconv"
	"strings"

	"keylint/internal/features/clipboard"
)

type xdotoolDesktop struct{ clip *clipboard.Service }

// NewDesktop is the development desktop (Linux, X11): xdotool for the windows,
// best-effort like the clipboard service's copy and paste. Without xdotool the
// source window is unknown, so a finished fix stays on the clipboard.
func NewDesktop(clip *clipboard.Service) Desktop { return xdotoolDesktop{clip: clip} }

func (xdotoolDesktop) Foreground() Window {
	out, err := exec.Command("xdotool", "getactivewindow").Output()
	if err != nil {
		return Window{}
	}
	id, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return Window{}
	}
	return Window{Handle: uintptr(id), PID: windowPID(id)}
}

func (xdotoolDesktop) Exists(w Window) bool {
	if !w.known() {
		return false
	}
	if err := exec.Command("xdotool", "getwindowname", strconv.FormatUint(uint64(w.Handle), 10)).Run(); err != nil {
		return false
	}
	return windowPID(uint64(w.Handle)) == w.PID
}

func (d xdotoolDesktop) Copy() error      { return d.clip.CopyFromForeground() }
func (d xdotoolDesktop) SendPaste() error { return d.clip.SendPaste() }

// windowPID is 0 when xdotool cannot say, on both sides of the comparison.
func windowPID(id uint64) uint32 {
	out, err := exec.Command("xdotool", "getwindowpid", strconv.FormatUint(id, 10)).Output()
	if err != nil {
		return 0
	}
	pid, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 32)
	if err != nil {
		return 0
	}
	return uint32(pid)
}
