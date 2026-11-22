// Package atomicfile replaces a file through a synced temporary file beside
// it, so a reader sees the old content or the new, never a part of either.
package atomicfile

import (
	"os"
	"path/filepath"
)

// Hooks inject failures between the steps of WriteTemp, for tests.
type Hooks struct {
	AfterWrite func() error
	AfterSync  func() error
	AfterClose func() error
}

func run(hook func() error) error {
	if hook == nil {
		return nil
	}
	return hook()
}

// WriteTemp writes data with mode to a synced, closed temporary file in dir,
// named by pattern. It returns the temporary path whenever it created one, so
// the caller removes it on every path.
func WriteTemp(dir, pattern string, mode os.FileMode, data []byte, hooks Hooks) (string, error) {
	temporary, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	path := temporary.Name()
	steps := []func() error{
		func() error { return temporary.Chmod(mode) },
		func() error { _, err := temporary.Write(data); return err },
		func() error { return run(hooks.AfterWrite) },
		temporary.Sync,
		func() error { return run(hooks.AfterSync) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			_ = temporary.Close()
			return path, err
		}
	}
	if err := temporary.Close(); err != nil {
		return path, err
	}
	return path, run(hooks.AfterClose)
}

// SyncDir makes a rename inside dir durable.
func SyncDir(dir string) error {
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return err
	}
	return directory.Close()
}

// Replace writes data to path with mode through a temporary file named by
// pattern, renames it into place and syncs the directory.
func Replace(path, pattern string, data []byte, mode os.FileMode) error {
	return ReplaceWith(path, pattern, data, mode, os.Rename)
}

// ReplaceWith is Replace with the rename step supplied by the caller.
func ReplaceWith(path, pattern string, data []byte, mode os.FileMode, rename func(string, string) error) error {
	dir := filepath.Dir(path)
	temporary, err := WriteTemp(dir, pattern, mode, data, Hooks{})
	if temporary != "" {
		defer os.Remove(temporary)
	}
	if err != nil {
		return err
	}
	if err := rename(temporary, path); err != nil {
		return err
	}
	return SyncDir(dir)
}
