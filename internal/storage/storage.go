package storage

import (
	"fmt"
	"strings"
	"time"

	"kode-stream/internal/ai"
	"kode-stream/internal/audit"
	"kode-stream/internal/common/models"
	appgit "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/knowledge"
	"kode-stream/internal/navigation"
	"kode-stream/internal/system"
	"kode-stream/internal/workspace/registry"
)

const (
	EnvStorageDriver = "KODE_STREAM_STORAGE_DRIVER"
	EnvSQLitePath    = "KODE_STREAM_SQLITE_PATH"
	EnvDatabaseURL   = "KODE_STREAM_DATABASE_URL"
	EnvMigrations    = "KODE_STREAM_MIGRATIONS"

	StorageDriverFile     = "file"
	StorageDriverSQLite   = "sqlite"
	StorageDriverPostgres = "postgres"
)

type Config struct {
	Driver      string
	SQLitePath  string
	DatabaseURL string
	Migrations  string
}

type AppOwnedState struct {
	Config        Config
	Workspaces    registry.Repository
	Items         itemindex.Repository
	ImportStatus  ImportStatusRepository
	Audit         audit.Repository
	Navigation    navigation.Repository
	AISettings    ai.SettingsStore
	Knowledge     *knowledge.Store
	LegacyFiles   system.Paths
	SQLStore      *SQLStore
	SQLiteStore   *SQLiteStore
	PostgresStore *PostgresStore
}

type ImportStatusRepository interface {
	ImportCompleted(string) (bool, error)
	MarkImportCompleted(string, time.Time) error
}

type SQLStore struct {
	driver string
}

type SQLiteStore struct {
	*SQLStore
	path string
}

type PostgresStore struct {
	*SQLStore
	url string
}

func ResolveConfig(runtime system.RuntimeConfig, paths system.Paths, getenv func(string) string) (Config, error) {
	driver := strings.ToLower(strings.TrimSpace(getenv(EnvStorageDriver)))
	if driver == "" {
		if runtime.Mode == models.RuntimeModeCloud {
			driver = StorageDriverPostgres
		} else {
			driver = StorageDriverSQLite
		}
	}
	config := Config{
		Driver:      driver,
		SQLitePath:  strings.TrimSpace(getenv(EnvSQLitePath)),
		DatabaseURL: strings.TrimSpace(getenv(EnvDatabaseURL)),
		Migrations:  strings.ToLower(strings.TrimSpace(getenv(EnvMigrations))),
	}
	if config.Migrations == "" {
		config.Migrations = "auto"
	}
	if config.SQLitePath == "" {
		config.SQLitePath = paths.SQLiteDatabaseFile
	}
	if config.Migrations != "auto" && config.Migrations != "manual" {
		return Config{}, fmt.Errorf("%s must be auto or manual", EnvMigrations)
	}
	switch config.Driver {
	case StorageDriverFile, StorageDriverSQLite, StorageDriverPostgres:
	default:
		return Config{}, fmt.Errorf("%s must be file, sqlite, or postgres", EnvStorageDriver)
	}
	if runtime.Mode == models.RuntimeModeCloud && config.Driver != StorageDriverPostgres {
		return Config{}, fmt.Errorf("cloud mode requires %s=postgres", EnvStorageDriver)
	}
	if config.Driver == StorageDriverPostgres && config.DatabaseURL == "" {
		return Config{}, fmt.Errorf("%s=postgres requires %s", EnvStorageDriver, EnvDatabaseURL)
	}
	return config, nil
}

func OpenAppOwnedState(paths system.Paths, runtime system.RuntimeConfig, git *appgit.GitAdapter, getenv func(string) string) (*AppOwnedState, error) {
	config, err := ResolveConfig(runtime, paths, getenv)
	if err != nil {
		return nil, err
	}
	state := &AppOwnedState{
		Config:      config,
		Workspaces:  registry.New(paths.RegistryFile, git),
		Items:       itemindex.New(paths.PlanIndexFile),
		Audit:       audit.New(paths.AuditLogFile),
		Navigation:  navigation.New(paths.SavedFiltersFile, paths.RecentItemsFile),
		AISettings:  ai.NewSettingsRepository(paths.AISettingsFile),
		Knowledge:   knowledge.NewStore(paths.KnowledgeIndexFile),
		LegacyFiles: paths,
	}
	switch config.Driver {
	case StorageDriverSQLite:
		state.SQLStore = &SQLStore{driver: StorageDriverSQLite}
		state.SQLiteStore = &SQLiteStore{SQLStore: state.SQLStore, path: config.SQLitePath}
	case StorageDriverPostgres:
		state.SQLStore = &SQLStore{driver: StorageDriverPostgres}
		state.PostgresStore = &PostgresStore{SQLStore: state.SQLStore, url: config.DatabaseURL}
	}
	return state, nil
}
