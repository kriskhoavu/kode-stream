//go:build !windows

package ptysession

import (
	"os"
	"syscall"
)

func stopProcess(process *os.Process) error {
	return syscall.Kill(-process.Pid, syscall.SIGKILL)
}
