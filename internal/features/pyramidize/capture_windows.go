//go:build windows

package pyramidize

import (
	"runtime"
	"syscall"
	"unsafe"
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procIsIconic                 = user32.NewProc("IsIconic")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procShowWindowAsync          = user32.NewProc("ShowWindowAsync")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
	procGetCurrentThreadID       = kernel32.NewProc("GetCurrentThreadId")
)

const swRestore = 9

// windowTitle is the window's title, "" if it has none or is gone.
func windowTitle(hwnd uintptr) string {
	buf := make([]uint16, 256)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}

// isVisible is false for a window hidden to the tray (Slack, Teams): it
// still exists, but a paste into it would land where the user cannot see.
func isVisible(hwnd uintptr) bool {
	r, _, _ := procIsWindowVisible.Call(hwnd)
	return r != 0
}

func isMinimised(hwnd uintptr) bool {
	r, _, _ := procIsIconic.Call(hwnd)
	return r != 0
}

// activateWindow restores hwnd if it is minimised and asks for it to become
// the foreground window.
//
// Send Back is a click in KeyLint's own window, so KeyLint is the foreground
// process and SetForegroundWindow is allowed. If Windows refuses anyway (the
// foreground lock can still apply, e.g. after a keystroke into another
// process), the input queue is attached to the foreground window's thread for
// the duration of a second attempt: with a shared input state, the caller
// counts as part of the foreground and the switch goes through. The caller
// checks the result either way and pastes nothing if it did not.
func activateWindow(hwnd uintptr) {
	if isMinimised(hwnd) {
		// Async: ShowWindow on another process's window waits for that
		// window to handle the message, and a hung app would hang Send Back
		// with it. The caller waits until the window is restored.
		procShowWindowAsync.Call(hwnd, swRestore)
	}
	if ok, _, _ := procSetForegroundWindow.Call(hwnd); ok != 0 {
		return
	}

	// AttachThreadInput and GetCurrentThreadId are about the OS thread, so
	// the goroutine must not move between the attach and the detach.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	fg, _, _ := procGetForegroundWindow.Call()
	var fgThread uintptr
	if fg != 0 {
		fgThread, _, _ = procGetWindowThreadProcessID.Call(fg, 0)
	}
	self, _, _ := procGetCurrentThreadID.Call()
	if fgThread != 0 && fgThread != self {
		if ok, _, _ := procAttachThreadInput.Call(self, fgThread, 1); ok != 0 {
			defer procAttachThreadInput.Call(self, fgThread, 0)
		}
	}
	// No BringWindowToTop: it is a synchronous cross-thread SetWindowPos and
	// would hang on a frozen app, and SetForegroundWindow raises the window.
	procSetForegroundWindow.Call(hwnd)
}
