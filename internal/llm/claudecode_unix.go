//go:build !windows

package llm

import (
	"os/exec"
	"syscall"
)

// configureCLIProcess puts the CLI in its own process group and kills the whole
// group on cancel. The npm-installed `claude` is a wrapper that spawns node, so
// killing only the direct child would leave the real work running.
func configureCLIProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// A negative PID addresses the process group.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
