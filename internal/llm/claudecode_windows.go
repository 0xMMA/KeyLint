//go:build windows

package llm

import (
	"os/exec"
	"strconv"
	"syscall"
)

// configureCLIProcess hides the console window — KeyLint runs from the tray, and
// a flashing console on every hotkey press would be unacceptable — and kills the
// whole process tree on cancel, because claude.cmd is an npm shim that spawns
// node and dies without taking it along.
func configureCLIProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		kill := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
		kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := kill.Run(); err != nil {
			// taskkill can be missing or refuse; the direct child is still ours.
			return cmd.Process.Kill()
		}
		return nil
	}
}
