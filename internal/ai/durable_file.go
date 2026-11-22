package ai

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"kode-stream/internal/filesystem/atomicfile"
)

const maxAIYAMLBytes int64 = 2 << 20

type durableFileOps struct {
	syncDirectory func(string) error
	remove        func(string) error
	before        func(string) error
}

func (o durableFileOps) check(phase string) error {
	if o.before == nil {
		return nil
	}
	return o.before(phase)
}

func readBoundedFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxAIYAMLBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxAIYAMLBytes {
		return nil, fmt.Errorf("AI persistence file exceeds %d byte limit", maxAIYAMLBytes)
	}
	return data, nil
}

// publishDurableFile is the local-store generation boundary. It preserves an
// operator-selected mode, syncs both file and containing directory, and restores
// the previous generation if directory durability confirmation fails.
func publishDurableFile(path string, data []byte) error {
	return publishDurableFileWith(path, data, durableFileOps{syncDirectory: syncAIDirectory, remove: os.Remove})
}

func publishDurableFileWith(path string, data []byte, ops durableFileOps) error {
	if ops.syncDirectory == nil {
		ops.syncDirectory = syncAIDirectory
	}
	if ops.remove == nil {
		ops.remove = os.Remove
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	previous, err := readBoundedFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	existed := err == nil
	if existed {
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
	}
	if err := ops.check("write"); err != nil {
		return err
	}
	hooks := atomicfile.Hooks{
		AfterWrite: func() error { return ops.check("sync") },
		AfterSync:  func() error { return ops.check("close") },
		AfterClose: func() error { return ops.check("rename") },
	}
	temporaryPath, err := atomicfile.WriteTemp(directory, ".ai-generation-*", mode, data, hooks)
	if temporaryPath != "" {
		defer os.Remove(temporaryPath)
	}
	if err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	err = ops.syncDirectory(directory)
	if err == nil {
		return nil
	}
	// Publication is not reported successful until the directory is durable. Best
	// effort rollback retains the original exact generation (or removes a first one).
	if existed {
		if rollbackErr := restoreDurableFile(path, previous, mode); rollbackErr != nil {
			return fmt.Errorf("publish AI persistence generation: %w (rollback failed: %v)", err, rollbackErr)
		}
	} else {
		if rollbackErr := ops.remove(path); rollbackErr != nil && !errors.Is(rollbackErr, os.ErrNotExist) {
			return fmt.Errorf("publish AI persistence generation: %w (rollback failed: %v)", err, rollbackErr)
		}
		if rollbackErr := ops.syncDirectory(directory); rollbackErr != nil {
			return fmt.Errorf("publish AI persistence generation: %w (rollback directory sync failed: %v)", err, rollbackErr)
		}
	}
	return fmt.Errorf("publish AI persistence generation: %w", err)
}

func syncAIDirectory(directory string) error {
	return atomicfile.SyncDir(directory)
}

func restoreDurableFile(path string, data []byte, mode os.FileMode) error {
	return atomicfile.Replace(path, ".ai-rollback-*", data, mode)
}
