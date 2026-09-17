//go:build windows

package llm

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// createNoWindow is CREATE_NO_WINDOW. syscall does not export it. Unlike
// HideWindow, which only asks the child to start hidden, this gives the process
// no console at all — and claude.cmd starts two console applications (cmd.exe
// and node), each of which would otherwise be a chance for a window to flash.
const createNoWindow = 0x08000000

// killTimeout bounds the kill itself. cmd.Cancel runs before Go starts the
// WaitDelay timer, so a taskkill that hangs would hang the whole cancellation
// and defeat the caller's deadline.
const killTimeout = 5 * time.Second

// configureCLIProcess suppresses the console window — KeyLint runs from the
// tray, and a console flashing on every hotkey press would be unacceptable —
// and kills the whole process tree on cancel, because claude.cmd is an npm shim
// that spawns node and dies without taking it along.
func configureCLIProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	cmd.Cancel = func() error {
		// Reading cmd.ProcessState here would race with Cmd.Wait, which writes
		// it while this runs; cmd.Process is set before this can be called.
		if cmd.Process == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), killTimeout)
		defer cancel()

		// /T walks the child tree (cmd.exe → node), which is the whole point.
		kill := exec.CommandContext(ctx, taskkillPath(), "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
		kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
		if err := kill.Run(); err != nil {
			// taskkill can be missing or refuse; the direct child is still ours,
			// even though node may then outlive it.
			return cmd.Process.Kill()
		}
		return nil
	}
}

// taskkillPath resolves taskkill absolutely so PATH order cannot decide which
// binary gets to kill processes.
func taskkillPath() string {
	if root := os.Getenv("SystemRoot"); root != "" {
		return filepath.Join(root, "System32", "taskkill.exe")
	}
	return "taskkill"
}
