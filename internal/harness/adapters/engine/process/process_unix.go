//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

// Prepare puts the command in a process group of its own.
func Prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func signalGroup(pid int, kill bool) error {
	signal := syscall.SIGTERM
	if kill {
		signal = syscall.SIGKILL
	}
	return syscall.Kill(-pid, signal)
}
