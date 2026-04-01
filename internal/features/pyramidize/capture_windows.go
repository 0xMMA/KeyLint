//go:build windows

package pyramidize

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	getForegroundWindow = user32.NewProc("GetForegroundWindow")
	getWindowTextW      = user32.NewProc("GetWindowTextW")
	setForegroundWindow = user32.NewProc("SetForegroundWindow")
	sendInput           = user32.NewProc("SendInput")
)

// captureSourceApp returns the title and HWND (as string) of the currently
// focused foreground window using Win32 APIs. Returns empty strings on failure.
func captureSourceApp() (name string, windowID string) {
	hwnd, _, _ := getForegroundWindow.Call()
	if hwnd == 0 {
		return "", ""
	}
	buf := make([]uint16, 256)
	getWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
	return syscall.UTF16ToString(buf), fmt.Sprintf("%d", hwnd)
}

// sendBackToWindow focuses the window identified by windowIDStr (a stringified HWND)
// and synthesises a Ctrl+V key sequence via SendInput.
func sendBackToWindow(windowIDStr string) error {
	if windowIDStr == "" {
		return nil
	}
	var hwnd uintptr
	fmt.Sscanf(windowIDStr, "%d", &hwnd)
	if hwnd == 0 {
		return nil
	}
	setForegroundWindow.Call(hwnd)

	// keyInput mirrors the Win32 INPUT structure for keyboard events.
	type keyboardInput struct {
		VK        uint16
		Scan      uint16
		Flags     uint32
		Time      uint32
		ExtraInfo uintptr
	}
	type input struct {
		Type    uint32
		Ki      keyboardInput
		Padding [8]byte
	}

	const (
		inputKeyboard   = 2
		keyeventfKeyup  = 0x0002
		vkControl       = 0x11
		vkV             = 0x56
	)

	inputs := [4]input{
		{Type: inputKeyboard, Ki: keyboardInput{VK: vkControl}},
		{Type: inputKeyboard, Ki: keyboardInput{VK: vkV}},
		{Type: inputKeyboard, Ki: keyboardInput{VK: vkV, Flags: keyeventfKeyup}},
		{Type: inputKeyboard, Ki: keyboardInput{VK: vkControl, Flags: keyeventfKeyup}},
	}

	sendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
	return nil
}
