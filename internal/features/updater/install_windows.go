//go:build windows

package updater

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	shell32             = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW   = shell32.NewProc("ShellExecuteW")
)

// applyPlatformUpdate launches the downloaded NSIS installer with UAC elevation
// and signals the app to quit. Go's exec.Command uses CreateProcess which cannot
// trigger UAC — ShellExecuteW with "runas" is required for elevated installers.
func applyPlatformUpdate(svc *Service, tmpPath string) (InstallResult, error) {
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(tmpPath)

	ret, _, _ := procShellExecuteW.Call(
		0, // hwnd
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		0, // parameters
		0, // directory
		uintptr(syscall.SW_SHOWNORMAL),
	)

	// ShellExecuteW returns a value > 32 on success.
	if ret <= 32 {
		os.Remove(tmpPath)
		return InstallResult{}, fmt.Errorf("launching installer: ShellExecute returned %d", ret)
	}

	// Schedule app quit so the installer can replace the locked executable.
	// The quitFunc is set by main.go and includes a short delay for the
	// frontend to display the "closing" message before the app exits.
	if svc.quitFunc != nil {
		go svc.quitFunc()
	}

	return InstallResult{RestartRequired: true}, nil
}
