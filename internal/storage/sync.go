package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"kode-stream/internal/common/models"
	appgit "kode-stream/internal/git"
	"kode-stream/internal/system"
)

const (
	SyncDataDirToDatabase = "datadir_to_database"
	SyncDatabaseToDataDir = "database_to_datadir"
)

type StorageSyncRequest struct {
	Direction string `json:"direction"`
	Confirm   bool   `json:"confirm"`
}

type StorageSyncResult struct {
	OK            bool           `json:"ok"`
	Direction     string         `json:"direction"`
	BackupPath    string         `json:"backupPath"`
	Summary       map[string]int `json:"summary"`
	Warnings      []string       `json:"warnings"`
	SkippedStores []string       `json:"skippedStores"`
}

type StorageSyncService struct {
	config      Config
	paths       system.Paths
	runtime     system.RuntimeConfig
	git         *appgit.GitAdapter
	running     atomic.Bool
	beforePhase func(string) error
}

type SyncErrorKind string

const (
	SyncErrorValidation     SyncErrorKind = "validation"
	SyncErrorConflict       SyncErrorKind = "conflict"
	SyncErrorInfrastructure SyncErrorKind = "infrastructure"
)

type SyncError struct {
	Kind SyncErrorKind
	Err  error
}

func (e *SyncError) Error() string { return e.Err.Error() }
func (e *SyncError) Unwrap() error { return e.Err }

func NewStorageSyncService(config Config, paths system.Paths, runtime system.RuntimeConfig, git *appgit.GitAdapter) *StorageSyncService {
	return &StorageSyncService{config: config, paths: paths, runtime: runtime, git: git}
}

func (s *StorageSyncService) Sync(ctx context.Context, request StorageSyncRequest) (StorageSyncResult, error) {
	if !s.running.CompareAndSwap(false, true) {
		return StorageSyncResult{}, &SyncError{Kind: SyncErrorConflict, Err: errors.New("storage sync is already in progress")}
	}
	defer s.running.Store(false)
	direction, err := s.validateSync(request)
	if err != nil {
		return StorageSyncResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return StorageSyncResult{}, err
	}
	result := StorageSyncResult{OK: true, Direction: direction, Summary: map[string]int{}, Warnings: []string{}, SkippedStores: []string{"knowledge"}}
	sync := s.syncToDatabase
	if direction == SyncDatabaseToDataDir {
		sync = s.syncToDataDir
	}
	snapshot, err := sync(ctx, &result)
	if err != nil {
		return StorageSyncResult{}, err
	}
	result.Summary = snapshot.summary()
	return result, nil
}

func (s *StorageSyncService) validateSync(request StorageSyncRequest) (string, error) {
	direction := strings.ToLower(strings.TrimSpace(request.Direction))
	if direction != SyncDataDirToDatabase && direction != SyncDatabaseToDataDir {
		return "", &SyncError{Kind: SyncErrorValidation, Err: fmt.Errorf("direction must be %s or %s", SyncDataDirToDatabase, SyncDatabaseToDataDir)}
	}
	if !request.Confirm {
		return "", &SyncError{Kind: SyncErrorValidation, Err: errors.New("storage sync requires confirmation")}
	}
	if s.runtime.Mode == models.RuntimeModeCloud {
		return "", &SyncError{Kind: SyncErrorValidation, Err: errors.New("storage sync is only available in local mode")}
	}
	if s.config.Driver == StorageDriverPostgres {
		return "", &SyncError{Kind: SyncErrorValidation, Err: errors.New("manual synchronization with Local Postgres is not supported")}
	}
	expected := SyncDataDirToDatabase
	if s.config.StorageOption == StorageOptionDatabase {
		expected = SyncDatabaseToDataDir
	}
	if direction != expected {
		return "", &SyncError{Kind: SyncErrorConflict, Err: fmt.Errorf("sync target is the active %s backend", s.config.StorageOption)}
	}
	return direction, nil
}

func (s *StorageSyncService) syncToDatabase(ctx context.Context, result *StorageSyncResult) (storageSnapshot, error) {
	snapshot, err := readDataDirSnapshot(s.paths)
	if err != nil {
		return snapshot, err
	}
	if err := ctx.Err(); err != nil {
		return snapshot, err
	}
	target := s.config.SQLitePath
	if target == "" {
		target = s.paths.SQLiteDatabaseFile
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return snapshot, &SyncError{Kind: SyncErrorInfrastructure, Err: err}
	}
	stage, err := os.CreateTemp(filepath.Dir(target), ".storage-sync-*.db")
	if err != nil {
		return snapshot, &SyncError{Kind: SyncErrorInfrastructure, Err: err}
	}
	stagePath := stage.Name()
	_ = stage.Close()
	_ = os.Remove(stagePath)
	defer os.Remove(stagePath)
	if err := s.writeDatabaseSnapshotAt(snapshot, stagePath); err != nil {
		return snapshot, &SyncError{Kind: SyncErrorInfrastructure, Err: err}
	}
	if err := ctx.Err(); err != nil {
		return snapshot, err
	}
	if err := s.phase(ctx, "before_backup"); err != nil {
		return snapshot, err
	}
	if result.BackupPath, err = backupDatabaseTarget(s.paths, target, result.Direction, time.Now().UTC()); err != nil {
		return snapshot, &SyncError{Kind: SyncErrorInfrastructure, Err: err}
	}
	if err := s.phase(ctx, "before_publish"); err != nil {
		return snapshot, err
	}
	if err := os.Rename(stagePath, target); err != nil {
		return snapshot, &SyncError{Kind: SyncErrorInfrastructure, Err: err}
	}
	return snapshot, nil
}

func (s *StorageSyncService) syncToDataDir(ctx context.Context, result *StorageSyncResult) (storageSnapshot, error) {
	snapshot, err := s.readDatabaseSnapshot()
	if err != nil {
		return snapshot, err
	}
	if err := ctx.Err(); err != nil {
		return snapshot, err
	}
	stageDir, err := os.MkdirTemp(s.paths.Dir, ".storage-sync-stage-*")
	if err != nil {
		return snapshot, &SyncError{Kind: SyncErrorInfrastructure, Err: err}
	}
	defer os.RemoveAll(stageDir)
	stagePaths := syncStagePaths(s.paths, stageDir)
	if err := writeDataDirSnapshot(stagePaths, snapshot); err != nil {
		return snapshot, &SyncError{Kind: SyncErrorInfrastructure, Err: err}
	}
	if err := s.phase(ctx, "before_backup"); err != nil {
		return snapshot, err
	}
	if result.BackupPath, err = backupDataDirTarget(s.paths, result.Direction, time.Now().UTC()); err != nil {
		return snapshot, &SyncError{Kind: SyncErrorInfrastructure, Err: err}
	}
	if err := ctx.Err(); err != nil {
		return snapshot, err
	}
	if err := s.phase(ctx, "before_publish"); err != nil {
		return snapshot, err
	}
	err = publishDataDirStage(stagePaths, s.paths, result.BackupPath, func(phase string) error {
		if strings.HasPrefix(phase, "restore:") {
			if s.beforePhase != nil {
				return s.beforePhase(phase)
			}
			return nil
		}
		return s.phase(ctx, phase)
	})
	if err != nil {
		return snapshot, &SyncError{Kind: SyncErrorInfrastructure, Err: err}
	}
	return snapshot, nil
}

func (s *StorageSyncService) phase(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.beforePhase != nil {
		return s.beforePhase(name)
	}
	return nil
}
