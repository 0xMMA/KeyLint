//go:build windows

package silentfix

import (
	"syscall"
	"unsafe"

	"keylint/internal/features/clipboard"
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procIsWindow                 = user32.NewProc("IsWindow")
)

type windowsDesktop struct{ clip *clipboard.Service }

// NewDesktop is the Windows desktop: Win32 for the windows, the clipboard
// service's SendInput for copy and paste (it releases the shortcut's held
// modifiers first, which a bare Ctrl+V would not).
func NewDesktop(clip *clipboard.Service) Desktop { return windowsDesktop{clip: clip} }

func (windowsDesktop) Foreground() Window {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return Window{}
	}
	return Window{Handle: hwnd, PID: windowPID(hwnd)}
}

func (windowsDesktop) Exists(w Window) bool {
	if !w.known() {
		return false
	}
	if ok, _, _ := procIsWindow.Call(w.Handle); ok == 0 {
		return false
	}
	return windowPID(w.Handle) == w.PID
}

func (d windowsDesktop) Copy() error  { return d.clip.CopyFromForeground() }
func (d windowsDesktop) Paste() error { return d.clip.PasteToForeground() }

func windowPID(hwnd uintptr) uint32 {
	var pid uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid
}
