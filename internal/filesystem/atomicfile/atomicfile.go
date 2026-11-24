// Package atomicfile replaces a file through a synced temporary file, so a reader never sees a partial write.
package atomicfile

import (
	"os"
	"path/filepath"
)

type Hooks struct {
	AfterWrite         func() error
	AfterSync          func() error
	AfterClose         func() error
	AfterRename        func() error
	AfterDirectorySync func() error
}

func run(hook func() error) error {
	if hook == nil {
		return nil
	}
	return hook()
}

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

func SyncDir(dir string) error {
	return SyncRename(dir, Hooks{})
}

func SyncRename(dir string, hooks Hooks) error {
	if err := run(hooks.AfterRename); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return err
	}
	if err := run(hooks.AfterDirectorySync); err != nil {
		_ = directory.Close()
		return err
	}
	return directory.Close()
}

func Replace(path, pattern string, data []byte, mode os.FileMode) error {
	return ReplaceWith(path, pattern, data, mode, os.Rename)
}

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
