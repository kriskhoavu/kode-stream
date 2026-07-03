//go:build windows

package ptysession

import "os"

func stopProcess(process *os.Process) error {
	return process.Kill()
}
