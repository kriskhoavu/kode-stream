package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type runtimeChecker interface {
	CurrentExecutable() (string, error)
	ResolvePaths() (Paths, error)
	CanWriteDir(string) error
}

type gitChecker interface {
	Version() (string, error)
	IsRepository(string) (bool, error)
	RemoteURL(string, string) (string, error)
	LSRemote(string) error
}

type defaultRuntimeChecker struct{}

func (defaultRuntimeChecker) CurrentExecutable() (string, error) {
	return os.Executable()
}

func (defaultRuntimeChecker) ResolvePaths() (Paths, error) {
	return ResolvePaths()
}

func (defaultRuntimeChecker) CanWriteDir(dir string) error {
	f, err := os.CreateTemp(dir, ".doctor-write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

type defaultGitChecker struct {
	timeout time.Duration
}

func newDefaultGitChecker() defaultGitChecker {
	return defaultGitChecker{timeout: 15 * time.Second}
}

func (g defaultGitChecker) Version() (string, error) {
	out, err := g.run("", "git", "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (g defaultGitChecker) IsRepository(path string) (bool, error) {
	out, err := g.run("", "git", "-C", path, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

func (g defaultGitChecker) RemoteURL(path, remoteName string) (string, error) {
	out, err := g.run("", "git", "-C", path, "config", "--get", "remote."+remoteName+".url")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (g defaultGitChecker) LSRemote(remoteURL string) error {
	_, err := g.run("", "git", "ls-remote", "--heads", remoteURL)
	return err
}

func (g defaultGitChecker) run(dir string, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), g.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = filepath.Clean(dir)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return string(out), nil
}
