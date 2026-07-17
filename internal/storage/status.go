package storage

import (
	"context"

	"kode-stream/internal/common/models"
	"kode-stream/internal/system"
)

type StorageStatus struct {
	Mode                  models.RuntimeMode `json:"mode"`
	StorageOption         string             `json:"storageOption"`
	StorageDriver         string             `json:"storageDriver"`
	EnvironmentLocked     bool               `json:"environmentLocked"`
	StorageOptionEnv      string             `json:"storageOptionEnv"`
	StorageDriverEnv      string             `json:"storageDriverEnv"`
	DataDir               string             `json:"dataDir"`
	DatabasePath          string             `json:"databasePath,omitempty"`
	DatabaseURLConfigured bool               `json:"databaseUrlConfigured"`
	Database              *DatabaseHealth    `json:"database,omitempty"`
}

type StorageStatusService struct {
	config  Config
	paths   system.Paths
	runtime system.RuntimeConfig
	sql     *SQLStore
}

func NewStorageStatusService(config Config, paths system.Paths, runtime system.RuntimeConfig, sql *SQLStore) *StorageStatusService {
	return &StorageStatusService{config: config, paths: paths, runtime: runtime, sql: sql}
}

func (s *StorageStatusService) Status(ctx context.Context) StorageStatus {
	status := StorageStatus{
		Mode:                  s.runtime.Mode,
		StorageOption:         s.config.StorageOption,
		StorageDriver:         s.config.Driver,
		EnvironmentLocked:     s.config.EnvironmentLock,
		StorageOptionEnv:      EnvStorageOption,
		StorageDriverEnv:      EnvStorageDriver,
		DataDir:               s.paths.Dir,
		DatabasePath:          s.config.SQLitePath,
		DatabaseURLConfigured: s.config.DatabaseURL != "",
	}
	if s.config.Driver == StorageDriverPostgres {
		status.DatabasePath = ""
	}
	if s.sql != nil {
		health := s.sql.Health(ctx)
		status.Database = &health
	}
	return status
}
