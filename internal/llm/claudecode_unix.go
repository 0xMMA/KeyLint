//go:build !windows

package llm

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// configureCLIProcess puts the CLI in its own process group and takes the whole
// group down on cancel. The npm-installed `claude` is a wrapper that spawns
// node, so killing only the direct child would leave the real work running.
func configureCLIProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Kill the direct child through its handle first: that is what os/exec
		// does by default and it is safe against PID reuse. Reading
		// cmd.ProcessState here instead would race with Cmd.Wait, which writes
		// it while this runs.
		killErr := cmd.Process.Kill()

		// Then the group, which is the only way to reach the node process the
		// wrapper spawned. A negative PID addresses the group. This one does use
		// the raw PID, so in the sliver between the child being reaped and this
		// call it could in principle address a recycled group; SIGKILL to an
		// already-dead group is ESRCH, which is the outcome we wanted anyway.
		if groupErr := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); groupErr != nil &&
			!errors.Is(groupErr, syscall.ESRCH) {
			return groupErr
		}
		if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return killErr
		}
		return nil
	}
}
