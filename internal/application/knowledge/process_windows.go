//go:build windows

package knowledge

import "os/exec"

func configureProcess(_ *exec.Cmd) {}

func killProcess(command *exec.Cmd) {
	if command.Process != nil {
		_ = command.Process.Kill()
	}
}
