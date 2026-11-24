package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"kode-stream/internal/system"
)

func backupDatabaseTarget(paths system.Paths, databasePath, direction string, now time.Time) (string, error) {
	backupPath := storageBackupPath(paths, direction, now)
	if err := os.MkdirAll(backupPath, 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(databasePath); errors.Is(err, os.ErrNotExist) {
		return backupPath, nil
	} else if err != nil {
		return "", err
	}
	backupDatabase := filepath.Join(backupPath, filepath.Base(databasePath))
	db, err := sql.Open("sqlite", databasePath)
	if err == nil {
		_, snapshotErr := db.Exec(`VACUUM INTO ?`, backupDatabase)
		closeErr := db.Close()
		if snapshotErr == nil && closeErr == nil {
			return backupPath, nil
		}
		_ = os.Remove(backupDatabase)
	}
	// A corrupt target is still operator data. Keep its bytes and journals, so a failed repair can be undone.
	for _, suffix := range []string{"", "-wal", "-shm"} {
		source := databasePath + suffix
		if err := copyIfExists(source, filepath.Join(backupPath, filepath.Base(source))); err != nil {
			return "", err
		}
	}
	return backupPath, nil
}

func backupDataDirTarget(paths system.Paths, direction string, now time.Time) (string, error) {
	backupPath := storageBackupPath(paths, direction, now)
	if err := os.MkdirAll(backupPath, 0o755); err != nil {
		return "", err
	}
	for _, source := range []string{paths.RegistryFile, paths.PlanIndexFile, paths.AuditLogFile, paths.SavedFiltersFile, paths.RecentItemsFile, paths.AISettingsFile, paths.CanvasFile, paths.AISessionRecordsFile, paths.KnowledgeIndexFile} {
		if err := copyIfExists(source, filepath.Join(backupPath, filepath.Base(source))); err != nil {
			return "", err
		}
	}
	return backupPath, nil
}

func storageBackupPath(paths system.Paths, direction string, now time.Time) string {
	return filepath.Join(paths.Dir, "backups", "storage-sync", now.Format("20060102-150405.000000000")+"-"+direction)
}

func syncStagePaths(paths system.Paths, dir string) system.Paths {
	staged := paths
	staged.Dir = dir
	staged.RegistryFile = filepath.Join(dir, filepath.Base(paths.RegistryFile))
	staged.PlanIndexFile = filepath.Join(dir, filepath.Base(paths.PlanIndexFile))
	staged.AuditLogFile = filepath.Join(dir, filepath.Base(paths.AuditLogFile))
	staged.SavedFiltersFile = filepath.Join(dir, filepath.Base(paths.SavedFiltersFile))
	staged.RecentItemsFile = filepath.Join(dir, filepath.Base(paths.RecentItemsFile))
	staged.AISettingsFile = filepath.Join(dir, filepath.Base(paths.AISettingsFile))
	staged.CanvasFile = filepath.Join(dir, filepath.Base(paths.CanvasFile))
	staged.AISessionRecordsFile = filepath.Join(dir, filepath.Base(paths.AISessionRecordsFile))
	return staged
}

func publishDataDirStage(stage, target system.Paths, backup string, before func(string) error) error {
	pairs := [][2]string{{stage.RegistryFile, target.RegistryFile}, {stage.PlanIndexFile, target.PlanIndexFile}, {stage.AuditLogFile, target.AuditLogFile}, {stage.SavedFiltersFile, target.SavedFiltersFile}, {stage.RecentItemsFile, target.RecentItemsFile}, {stage.AISettingsFile, target.AISettingsFile}, {stage.CanvasFile, target.CanvasFile}, {stage.AISessionRecordsFile, target.AISessionRecordsFile}}
	for _, pair := range pairs {
		if before != nil {
			if err := before("publish:" + filepath.Base(pair[1])); err != nil {
				return rollbackDataDirPublication(pairs, backup, before, err)
			}
		}
		if err := os.Rename(pair[0], pair[1]); err != nil {
			return rollbackDataDirPublication(pairs, backup, before, err)
		}
	}
	return nil
}

func rollbackDataDirPublication(pairs [][2]string, backup string, before func(string) error, publishErr error) error {
	rollbackErrors := []error{fmt.Errorf("publish target generation: %w", publishErr)}
	for _, pair := range pairs {
		target := pair[1]
		if before != nil {
			if err := before("restore:" + filepath.Base(target)); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %s: %w", target, err))
				continue
			}
		}
		backupFile := filepath.Join(backup, filepath.Base(target))
		if _, statErr := os.Stat(backupFile); statErr == nil {
			if err := restoreFileAtomically(backupFile, target); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %s: %w", target, err))
			}
		} else if errors.Is(statErr, os.ErrNotExist) {
			if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("remove newly published %s: %w", target, err))
			}
		} else {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("inspect backup %s: %w", backupFile, statErr))
		}
	}
	return errors.Join(rollbackErrors...)
}

func restoreFileAtomically(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".storage-restore-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	in, err := os.Open(source)
	if err != nil {
		_ = temporary.Close()
		return err
	}
	_, copyErr := io.Copy(temporary, in)
	closeInErr := in.Close()
	if copyErr == nil {
		copyErr = temporary.Sync()
	}
	closeOutErr := temporary.Close()
	if err := errors.Join(copyErr, closeInErr, closeOutErr); err != nil {
		return err
	}
	return os.Rename(temporaryPath, target)
}

func copyIfExists(source, target string) error {
	in, err := os.Open(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
