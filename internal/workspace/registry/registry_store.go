package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"kode-stream/internal/common/models"
)

func (r *Registry) load() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loaded {
		return nil
	}
	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		r.records = []models.WorkspaceConfig{}
		r.loaded = true
		return nil
	}
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, &r.records); err != nil {
		return err
	}
	r.loaded = true
	return nil
}

func (r *Registry) commitLocked(records []models.WorkspaceConfig) error {
	if err := r.saveRecordsLocked(records); err != nil {
		return err
	}
	r.records = records
	return nil
}

func (r *Registry) saveRecordsLocked(records []models.WorkspaceConfig) error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(records)
	if err != nil {
		return err
	}
	previous, readErr := os.ReadFile(r.path)
	previousMode := os.FileMode(0o600)
	previousExists := readErr == nil
	if previousExists {
		if info, statErr := os.Stat(r.path); statErr == nil {
			previousMode = info.Mode().Perm()
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	temporary, err := os.CreateTemp(filepath.Dir(r.path), ".workspaces-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if r.hooks.afterWrite != nil {
		if err := r.hooks.afterWrite(); err != nil {
			_ = temporary.Close()
			return err
		}
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if r.hooks.afterSync != nil {
		if err := r.hooks.afterSync(); err != nil {
			_ = temporary.Close()
			return err
		}
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if r.hooks.afterClose != nil {
		if err := r.hooks.afterClose(); err != nil {
			return err
		}
	}
	if err := os.Rename(temporaryPath, r.path); err != nil {
		return err
	}
	rollback := func(cause error) error {
		if restoreErr := restoreRegistryGeneration(r.path, previous, previousMode, previousExists); restoreErr != nil {
			return fmt.Errorf("registry persistence failed: %v; previous generation restoration failed: %w", cause, restoreErr)
		}
		return cause
	}
	if r.hooks.afterRename != nil {
		if err := r.hooks.afterRename(); err != nil {
			return rollback(err)
		}
	}
	directory, err := os.Open(filepath.Dir(r.path))
	if err != nil {
		return rollback(err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return rollback(err)
	}
	if r.hooks.afterDirectorySync != nil {
		if err := r.hooks.afterDirectorySync(); err != nil {
			_ = directory.Close()
			return rollback(err)
		}
	}
	if err := directory.Close(); err != nil {
		return rollback(err)
	}
	return nil
}

func restoreRegistryGeneration(path string, data []byte, mode os.FileMode, existed bool) error {
	if !existed {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else {
		temporary, err := os.CreateTemp(filepath.Dir(path), ".workspaces-restore-*.tmp")
		if err != nil {
			return err
		}
		temporaryPath := temporary.Name()
		defer os.Remove(temporaryPath)
		if err := temporary.Chmod(mode); err != nil {
			_ = temporary.Close()
			return err
		}
		if _, err := temporary.Write(data); err != nil {
			_ = temporary.Close()
			return err
		}
		if err := temporary.Sync(); err != nil {
			_ = temporary.Close()
			return err
		}
		if err := temporary.Close(); err != nil {
			return err
		}
		if err := os.Rename(temporaryPath, path); err != nil {
			return err
		}
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
