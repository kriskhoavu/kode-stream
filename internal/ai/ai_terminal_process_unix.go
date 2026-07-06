//go:build !windows

package ai

import (
	"os"
	"syscall"
)

func stopProcess(process *os.Process) error {
	return syscall.Kill(-process.Pid, syscall.SIGKILL)
}
