//go:build windows

package process

import (
	"os"
	"os/exec"
)

func Prepare(*exec.Cmd) {}

func signalGroup(pid int, _ bool) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}
