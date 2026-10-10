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
	procGetGUIThreadInfo         = user32.NewProc("GetGUIThreadInfo")
	procIsWindow                 = user32.NewProc("IsWindow")
)

// guiThreadInfo mirrors GUITHREADINFO.
type guiThreadInfo struct {
	cbSize        uint32
	flags         uint32
	hwndActive    uintptr
	hwndFocus     uintptr
	hwndCapture   uintptr
	hwndMenuOwner uintptr
	hwndMoveSize  uintptr
	hwndCaret     uintptr
	rcCaret       struct{ left, top, right, bottom int32 }
}

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
	var pid uint32
	tid, _, _ := procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	w := Window{Handle: hwnd, PID: pid}
	// The control with the keyboard focus, where the window draws it as a
	// child window of its own (Win32 and WinForms fields, Office). Browsers
	// and Electron draw every field in one child, so for them this stays the
	// same while the user moves between fields — the top-level check is all
	// there is there.
	info := guiThreadInfo{}
	info.cbSize = uint32(unsafe.Sizeof(info))
	if ok, _, _ := procGetGUIThreadInfo.Call(tid, uintptr(unsafe.Pointer(&info))); ok != 0 {
		w.Focus = info.hwndFocus
	}
	return w
}

func (windowsDesktop) Exists(w Window) bool {
	if !w.known() {
		return false
	}
	if ok, _, _ := procIsWindow.Call(w.Handle); ok == 0 {
		return false
	}
	var pid uint32
	procGetWindowThreadProcessID.Call(w.Handle, uintptr(unsafe.Pointer(&pid)))
	return pid == w.PID
}

func (d windowsDesktop) Copy() error      { return d.clip.CopyFromForeground() }
func (d windowsDesktop) SendPaste() error { return d.clip.SendPaste() }
