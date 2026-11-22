package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/atomicfile"
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
	hooks := atomicfile.Hooks{AfterWrite: r.hooks.afterWrite, AfterSync: r.hooks.afterSync, AfterClose: r.hooks.afterClose, AfterRename: r.hooks.afterRename, AfterDirectorySync: r.hooks.afterDirectorySync}
	temporaryPath, err := atomicfile.WriteTemp(filepath.Dir(r.path), ".workspaces-*.tmp", 0o600, data, hooks)
	if temporaryPath != "" {
		defer os.Remove(temporaryPath)
	}
	if err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, r.path); err != nil {
		return err
	}
	if err := atomicfile.SyncRename(filepath.Dir(r.path), hooks); err != nil {
		if restoreErr := restoreRegistryGeneration(r.path, previous, previousMode, previousExists); restoreErr != nil {
			return fmt.Errorf("registry persistence failed: %v; previous generation restoration failed: %w", err, restoreErr)
		}
		return err
	}
	return nil
}

func restoreRegistryGeneration(path string, data []byte, mode os.FileMode, existed bool) error {
	if existed {
		return atomicfile.Replace(path, ".workspaces-restore-*.tmp", data, mode)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return atomicfile.SyncDir(filepath.Dir(path))
}
