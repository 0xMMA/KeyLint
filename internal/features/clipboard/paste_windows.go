//go:build windows

package clipboard

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"keylint/internal/logger"
)

var (
	clipUser32                  = syscall.NewLazyDLL("user32.dll")
	clipSendInput               = clipUser32.NewProc("SendInput")
	clipGetForegroundWindow     = clipUser32.NewProc("GetForegroundWindow")
	clipGetClipboardSeqNumber   = clipUser32.NewProc("GetClipboardSequenceNumber")
)

const (
	inputKeyboard = 1
	keyEventKeyUp = 0x0002
	vkShift       = 0x10
	vkControl     = 0x11
	vkMenu        = 0x12 // Alt
	vkC           = 0x43
	vkV           = 0x56
)

// pasteInput mirrors the Win32 INPUT struct (keyboard variant, 40 bytes on 64-bit).
// Go aligns uintptr to 8 bytes, so an explicit uint32 pad is needed before
// dwExtraInfo to avoid implicit padding that would push the total to 44 bytes.
// Layout: type(4)+pad(4)+wVk(2)+wScan(2)+dwFlags(4)+time(4)+pad(4)+dwExtraInfo(8)+pad(8) = 40
type pasteInput struct {
	inputType   uint32
	_           uint32 // pad: align union to 8 bytes
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	_           uint32  // pad: align dwExtraInfo to 8 bytes
	dwExtraInfo uintptr // must be at offset 24 to match KEYBDINPUT layout
	_           [8]byte // pad: union is 32 bytes (size of MOUSEINPUT)
}

// CopyFromForeground sends Ctrl+C to the foreground window via Win32 SendInput,
// then polls GetClipboardSequenceNumber until the clipboard changes (or timeout).
// This handles slow clipboard providers like Outlook that use delayed rendering.
func (s *Service) CopyFromForeground() error {
	hwnd, _, _ := clipGetForegroundWindow.Call()
	logger.Info("clipboard: CopyFromForeground sending Ctrl+C", "foreground_hwnd", hwnd)

	// Snapshot the clipboard sequence number before sending Ctrl+C.
	seqBefore, _, _ := clipGetClipboardSeqNumber.Call()

	// Release Shift and Alt before Ctrl+C — the user may still be holding modifier keys
	// from the shortcut combo (e.g. Ctrl+Shift+G). Without this, the target app sees
	// Ctrl+Shift+C instead of Ctrl+C, which Outlook and other apps ignore.
	inputs := [6]pasteInput{
		{inputType: inputKeyboard, wVk: vkShift, dwFlags: keyEventKeyUp, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkMenu, dwFlags: keyEventKeyUp, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkControl, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkC, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkC, dwFlags: keyEventKeyUp, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkControl, dwFlags: keyEventKeyUp, dwExtraInfo: 0x4B4C},
	}
	ret, _, err := clipSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
	if ret == 0 {
		logger.Error("clipboard: CopyFromForeground SendInput failed", "err", err)
		return fmt.Errorf("SendInput (Ctrl+C) failed: %w", err)
	}

	// Poll until the clipboard sequence number changes, up to 1s.
	// Apps like Outlook use delayed rendering and may take 200-500ms.
	const pollInterval = 25 * time.Millisecond
	const timeout = 1 * time.Second
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		seqNow, _, _ := clipGetClipboardSeqNumber.Call()
		if seqNow != seqBefore {
			logger.Info("clipboard: CopyFromForeground ok", "wait_ms", time.Since(deadline.Add(-timeout)).Milliseconds())
			return nil
		}
	}

	// Timeout — clipboard didn't change. Might still work (some apps don't update the sequence number).
	logger.Warn("clipboard: CopyFromForeground timed out waiting for clipboard change")
	return nil
}

// PasteToForeground sends Ctrl+V to the foreground window via Win32 SendInput.
// A 150 ms delay is applied first to let the clipboard write settle.
func (s *Service) PasteToForeground() error {
	time.Sleep(150 * time.Millisecond)
	hwnd, _, _ := clipGetForegroundWindow.Call()
	logger.Info("clipboard: PasteToForeground sending Ctrl+V", "foreground_hwnd", hwnd)
	// Release Shift and Alt before Ctrl+V (same reason as CopyFromForeground).
	inputs := [6]pasteInput{
		{inputType: inputKeyboard, wVk: vkShift, dwFlags: keyEventKeyUp, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkMenu, dwFlags: keyEventKeyUp, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkControl, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkV, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkV, dwFlags: keyEventKeyUp, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkControl, dwFlags: keyEventKeyUp, dwExtraInfo: 0x4B4C},
	}
	ret, _, err := clipSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
	if ret == 0 {
		logger.Error("clipboard: SendInput failed", "err", err)
		return fmt.Errorf("SendInput failed: %w", err)
	}
	logger.Info("clipboard: PasteToForeground ok", "inputs_sent", ret)
	return nil
}
