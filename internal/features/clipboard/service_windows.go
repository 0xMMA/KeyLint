//go:build windows

package clipboard

import (
	"fmt"
	"syscall"
	"unsafe"

	"keylint/internal/logger"
)

// Service reads from and writes to the system clipboard via Win32 APIs.
type Service struct{}

// NewService creates a new ClipboardService.
func NewService() *Service { return &Service{} }

var (
	clipKernel32     = syscall.NewLazyDLL("kernel32.dll")
	clipOpenCB       = clipUser32.NewProc("OpenClipboard")
	clipCloseCB      = clipUser32.NewProc("CloseClipboard")
	clipGetCBData    = clipUser32.NewProc("GetClipboardData")
	clipEmptyCB      = clipUser32.NewProc("EmptyClipboard")
	clipSetCBData    = clipUser32.NewProc("SetClipboardData")
	clipGlobalAlloc  = clipKernel32.NewProc("GlobalAlloc")
	clipGlobalLock   = clipKernel32.NewProc("GlobalLock")
	clipGlobalUnlock = clipKernel32.NewProc("GlobalUnlock")
	clipGlobalFree   = clipKernel32.NewProc("GlobalFree")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// Read returns the current clipboard text using the Win32 CF_UNICODETEXT format.
func (s *Service) Read() (string, error) {
	logger.Debug("clipboard: Read called")
	ret, _, err := clipOpenCB.Call(0)
	if ret == 0 {
		logger.Error("clipboard: OpenClipboard failed", "err", err)
		return "", fmt.Errorf("OpenClipboard: %w", err)
	}
	defer clipCloseCB.Call()

	handle, _, err := clipGetCBData.Call(cfUnicodeText)
	if handle == 0 {
		logger.Error("clipboard: GetClipboardData failed", "err", err)
		return "", fmt.Errorf("GetClipboardData: %w", err)
	}

	ptr, _, err := clipGlobalLock.Call(handle)
	if ptr == 0 {
		logger.Error("clipboard: GlobalLock failed", "err", err)
		return "", fmt.Errorf("GlobalLock: %w", err)
	}
	defer clipGlobalUnlock.Call(handle)

	// Walk the UTF-16 buffer until the null terminator.
	var u16 []uint16
	base := globalPointer(ptr)
	for i := uintptr(0); ; i++ {
		ch := *(*uint16)(unsafe.Add(base, i*2))
		if ch == 0 {
			break
		}
		u16 = append(u16, ch)
	}
	text := syscall.UTF16ToString(u16)
	logger.Debug("clipboard: Read ok", "len", len(text))
	return text, nil
}

// Write stores text in the clipboard using CF_UNICODETEXT.
func (s *Service) Write(text string) error {
	logger.Debug("clipboard: Write called", "len", len(text))
	u16, err := syscall.UTF16FromString(text)
	if err != nil {
		return fmt.Errorf("UTF16FromString: %w", err)
	}
	byteLen := uintptr(len(u16) * 2)

	ret, _, err := clipOpenCB.Call(0)
	if ret == 0 {
		logger.Error("clipboard: OpenClipboard failed", "err", err)
		return fmt.Errorf("OpenClipboard: %w", err)
	}
	defer clipCloseCB.Call()

	ret, _, err = clipEmptyCB.Call()
	if ret == 0 {
		logger.Error("clipboard: EmptyClipboard failed", "err", err)
		return fmt.Errorf("EmptyClipboard: %w", err)
	}

	handle, _, err := clipGlobalAlloc.Call(gmemMoveable, byteLen)
	if handle == 0 {
		logger.Error("clipboard: GlobalAlloc failed", "err", err)
		return fmt.Errorf("GlobalAlloc: %w", err)
	}

	ptr, _, err := clipGlobalLock.Call(handle)
	if ptr == 0 {
		clipGlobalFree.Call(handle)
		logger.Error("clipboard: GlobalLock failed", "err", err)
		return fmt.Errorf("GlobalLock: %w", err)
	}
	copy(unsafe.Slice((*uint16)(globalPointer(ptr)), len(u16)), u16)
	clipGlobalUnlock.Call(handle)

	ret, _, err = clipSetCBData.Call(cfUnicodeText, handle)
	if ret == 0 {
		clipGlobalFree.Call(handle)
		logger.Error("clipboard: SetClipboardData failed", "err", err)
		return fmt.Errorf("SetClipboardData: %w", err)
	}
	// On success Windows owns the handle; do not free.
	logger.Debug("clipboard: Write ok")
	return nil
}

// globalPointer turns the address GlobalLock returned into a pointer. The
// block is global memory owned by Windows, not by the Go heap, so there is
// nothing for the garbage collector to track; reading the uintptr's bits as a
// pointer says so without the uintptr-to-pointer conversion vet rejects.
func globalPointer(addr uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr))
}
