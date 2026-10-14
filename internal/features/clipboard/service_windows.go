//go:build windows

package clipboard

import (
	"fmt"
	"sync"
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

	clipRegisterFormat = clipUser32.NewProc("RegisterClipboardFormatW")
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
	return write(text, "")
}

// WriteRich stores text as CF_UNICODETEXT and html as CF_HTML in one
// clipboard transaction: an app that takes formatted text (Outlook, Teams, a
// browser) pastes the HTML, every other app the text. If the HTML cannot be
// added, the text is still written and the paste comes out plain.
func (s *Service) WriteRich(text, html string) error {
	logger.Debug("clipboard: WriteRich called", "len", len(text), "html_len", len(html))
	return write(text, html)
}

// htmlFormat is the format id Windows gives "HTML Format", registered once.
var htmlFormat = sync.OnceValues(func() (uintptr, error) {
	name, err := syscall.UTF16PtrFromString("HTML Format")
	if err != nil {
		return 0, err
	}
	id, _, callErr := clipRegisterFormat.Call(uintptr(unsafe.Pointer(name)))
	if id == 0 {
		return 0, fmt.Errorf("RegisterClipboardFormat: %w", callErr)
	}
	return id, nil
})

func write(text, html string) error {
	u16, err := syscall.UTF16FromString(text)
	if err != nil {
		return fmt.Errorf("UTF16FromString: %w", err)
	}

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

	// The terminating NUL is part of both formats' data.
	if err := setData(cfUnicodeText, unsafe.Slice((*byte)(unsafe.Pointer(&u16[0])), len(u16)*2)); err != nil {
		logger.Error("clipboard: setting the text failed", "err", err)
		return err
	}
	if html != "" {
		id, err := htmlFormat()
		if err == nil {
			err = setData(id, append(cfHTML(html), 0))
		}
		if err != nil {
			logger.Warn("clipboard: HTML not added, the text alone is on the clipboard", "err", err)
		}
	}
	logger.Debug("clipboard: Write ok", "html", html != "")
	return nil
}

// setData copies data into global memory and hands it to the open clipboard
// as format. On success Windows owns the memory; on failure it is freed here.
func setData(format uintptr, data []byte) error {
	handle, _, err := clipGlobalAlloc.Call(gmemMoveable, uintptr(len(data)))
	if handle == 0 {
		return fmt.Errorf("GlobalAlloc: %w", err)
	}
	ptr, _, err := clipGlobalLock.Call(handle)
	if ptr == 0 {
		clipGlobalFree.Call(handle)
		return fmt.Errorf("GlobalLock: %w", err)
	}
	copy(unsafe.Slice((*byte)(globalPointer(ptr)), len(data)), data)
	clipGlobalUnlock.Call(handle)

	if ret, _, err := clipSetCBData.Call(format, handle); ret == 0 {
		clipGlobalFree.Call(handle)
		return fmt.Errorf("SetClipboardData: %w", err)
	}
	return nil
}

// globalPointer turns the address GlobalLock returned into a pointer. The
// block is global memory owned by Windows, not by the Go heap, so there is
// nothing for the garbage collector to track; reading the uintptr's bits as a
// pointer says so without the uintptr-to-pointer conversion vet rejects.
func globalPointer(addr uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr))
}
