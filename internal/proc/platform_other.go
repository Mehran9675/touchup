//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
)

// platform starts cmd in a process group of its own, and makes
// cancelling it kill the whole group: engines start helpers, such as
// FFmpeg, that would otherwise keep running and hold its output open.
func platform(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
