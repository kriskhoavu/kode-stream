//go:build windows

package ai

import "os"

func stopProcess(process *os.Process) error {
	return process.Kill()
}
