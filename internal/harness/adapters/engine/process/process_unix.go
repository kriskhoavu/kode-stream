//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

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
