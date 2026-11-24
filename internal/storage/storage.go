package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"kode-stream/internal/ai"
	"kode-stream/internal/audit"
	"kode-stream/internal/canvas"
	"kode-stream/internal/cloudstate"
	"kode-stream/internal/common/models"
	appgit "kode-stream/internal/git"
	harnessdomain "kode-stream/internal/harness/domain"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/knowledge"
	"kode-stream/internal/navigation"
	"kode-stream/internal/system"
	"kode-stream/internal/workspace/registry"
)

const (
	EnvStorageOption = "KODE_STREAM_STORAGE_OPTION"
	EnvStorageDriver = "KODE_STREAM_STORAGE_DRIVER"
	EnvSQLitePath    = "KODE_STREAM_SQLITE_PATH"
	EnvDatabaseURL   = "KODE_STREAM_DATABASE_URL"
	EnvMigrations    = "KODE_STREAM_MIGRATIONS"

	StorageOptionDatabase = "database"
	StorageOptionDataDir  = "datadir"

	StorageDriverFile     = "file"
	StorageDriverSQLite   = "sqlite"
	StorageDriverPostgres = "postgres"
)

type Config struct {
	StorageOption    string
	Driver           string
	SQLitePath       string
	DatabaseURL      string
	Migrations       string
	EnvironmentLock  bool
	StorageOptionSet bool
	StorageDriverSet bool
}

type AppOwnedState struct {
	Config         Config
	Workspaces     registry.Repository
	Items          itemindex.Repository
	ImportStatus   ImportStatusRepository
	Audit          audit.Repository
	Navigation     navigation.Repository
	AISettings     ai.SettingsStore
	Canvas         canvas.Repository
	SessionRecords ai.SessionRecordRepository
	Knowledge      *knowledge.Store
	LegacyFiles    system.Paths
	SQLStore       *SQLStore
	SQLiteStore    *SQLiteStore
	PostgresStore  *PostgresStore
	Provider       StorageProvider
	Repositories   RepositoryBundle
	StatusService  *StorageStatusService
	SyncService    *StorageSyncService
	Cloud          cloudstate.Repository
	Harness        harnessdomain.Repository
}

type RepositoryBundle struct {
	Workspaces     registry.Repository
	Items          itemindex.Repository
	Audit          audit.Repository
	Navigation     navigation.Repository
	AISettings     ai.SettingsStore
	Canvas         canvas.Repository
	SessionRecords ai.SessionRecordRepository
	Knowledge      *knowledge.Store
}

type StorageProvider interface {
	Name() string
	Repositories() RepositoryBundle
	SQLStore() *SQLStore
	Close() error
}

type ImportStatusRepository interface {
	ImportCompleted(string) (bool, error)
	MarkImportCompleted(string, time.Time) error
}

type SQLStore struct {
	driver           string
	db               *sql.DB
	migrationVersion int
}

type SQLiteStore struct {
	*SQLStore
	path string
}

type PostgresStore struct {
	*SQLStore
	url string
}

type DatabaseHealth struct {
	Driver           string `json:"driver"`
	Configured       bool   `json:"configured"`
	OK               bool   `json:"ok"`
	MigrationVersion int    `json:"migrationVersion"`
	Error            string `json:"error,omitempty"`
}

func ResolveConfig(runtime system.RuntimeConfig, paths system.Paths, getenv func(string) string) (Config, error) {
	option := strings.ToLower(strings.TrimSpace(getenv(EnvStorageOption)))
	driver := strings.ToLower(strings.TrimSpace(getenv(EnvStorageDriver)))
	optionSet := option != ""
	driverSet := driver != ""
	if option == "" && paths.DefaultDir != "" {
		if persisted, err := system.ResolveStorageOptionOverride(paths.DefaultDir); err == nil {
			option = strings.ToLower(strings.TrimSpace(persisted))
		}
	}
	if option == "" {
		if runtime.Mode == models.RuntimeModeCloud {
			option = StorageOptionDatabase
		} else {
			option = StorageOptionDataDir
		}
	}
	switch option {
	case StorageOptionDatabase, StorageOptionDataDir:
	default:
		return Config{}, fmt.Errorf("%s must be database or datadir", EnvStorageOption)
	}
	if driver == "" {
		switch {
		case runtime.Mode == models.RuntimeModeCloud:
			driver = StorageDriverPostgres
		case option == StorageOptionDataDir:
			driver = StorageDriverFile
		default:
			driver = StorageDriverSQLite
		}
	}
	if driverSet {
		option = storageOptionForDriver(driver)
	}
	config := Config{
		StorageOption:    option,
		Driver:           driver,
		SQLitePath:       strings.TrimSpace(getenv(EnvSQLitePath)),
		DatabaseURL:      strings.TrimSpace(getenv(EnvDatabaseURL)),
		Migrations:       strings.ToLower(strings.TrimSpace(getenv(EnvMigrations))),
		EnvironmentLock:  optionSet || driverSet,
		StorageOptionSet: optionSet,
		StorageDriverSet: driverSet,
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
	if config.StorageOption == StorageOptionDatabase && config.Driver == StorageDriverFile {
		return Config{}, fmt.Errorf("%s=database cannot use %s=file", EnvStorageOption, EnvStorageDriver)
	}
	if config.StorageOption == StorageOptionDataDir && config.Driver != StorageDriverFile {
		return Config{}, fmt.Errorf("%s=datadir requires %s=file", EnvStorageOption, EnvStorageDriver)
	}
	if runtime.Mode == models.RuntimeModeCloud && config.Driver != StorageDriverPostgres {
		return Config{}, fmt.Errorf("cloud mode requires %s=database with %s=postgres", EnvStorageOption, EnvStorageDriver)
	}
	if config.Driver == StorageDriverPostgres && config.DatabaseURL == "" {
		return Config{}, fmt.Errorf("%s=postgres requires %s", EnvStorageDriver, EnvDatabaseURL)
	}
	return config, nil
}

func storageOptionForDriver(driver string) string {
	if driver == StorageDriverFile {
		return StorageOptionDataDir
	}
	return StorageOptionDatabase
}

func OpenAppOwnedState(paths system.Paths, runtime system.RuntimeConfig, git *appgit.GitAdapter, getenv func(string) string) (*AppOwnedState, error) {
	config, err := ResolveConfig(runtime, paths, getenv)
	if err != nil {
		return nil, err
	}
	state := &AppOwnedState{
		Config:         config,
		Workspaces:     registry.New(paths.RegistryFile, git),
		Items:          itemindex.New(paths.PlanIndexFile),
		Audit:          audit.New(paths.AuditLogFile),
		Navigation:     navigation.New(paths.SavedFiltersFile, paths.RecentItemsFile),
		AISettings:     ai.NewSettingsRepository(paths.AISettingsFile),
		Canvas:         canvas.NewFileRepository(paths.CanvasFile),
		SessionRecords: ai.NewFileSessionRecordRepository(paths.AISessionRecordsFile),
		Knowledge:      knowledge.NewStore(paths.KnowledgeIndexFile),
		LegacyFiles:    paths,
	}
	state.Repositories = RepositoryBundle{
		Workspaces:     state.Workspaces,
		Items:          state.Items,
		Audit:          state.Audit,
		Navigation:     state.Navigation,
		AISettings:     state.AISettings,
		Canvas:         state.Canvas,
		SessionRecords: state.SessionRecords,
		Knowledge:      state.Knowledge,
	}
	switch config.Driver {
	case StorageDriverFile:
		state.Provider = &DataDirProvider{repositories: state.Repositories}
	case StorageDriverSQLite:
		sqlStore, err := openSQLStore(config)
		if err != nil {
			return nil, err
		}
		state.SQLStore = sqlStore
		state.SQLiteStore = &SQLiteStore{SQLStore: state.SQLStore, path: config.SQLitePath}
		state.Workspaces = newSQLiteWorkspaceRepository(sqlStore, paths, git)
		state.Items = &SQLiteItemRepository{db: sqlStore.db, driver: sqlStore.driver}
		state.ImportStatus = &SQLiteImportStatusRepository{db: sqlStore.db, driver: sqlStore.driver}
		state.Audit = &SQLiteAuditRepository{db: sqlStore.db, driver: sqlStore.driver, now: time.Now}
		state.Navigation = &SQLiteNavigationRepository{db: sqlStore.db, driver: sqlStore.driver, now: time.Now}
		state.AISettings = &SQLiteAISettingsRepository{db: sqlStore.db, driver: sqlStore.driver}
		state.Canvas = &SQLiteCanvasRepository{db: sqlStore.db, driver: sqlStore.driver, now: time.Now}
		state.SessionRecords = &SQLiteSessionRecordRepository{db: sqlStore.db, driver: sqlStore.driver}
		state.Repositories = RepositoryBundle{Workspaces: state.Workspaces, Items: state.Items, Audit: state.Audit, Navigation: state.Navigation, AISettings: state.AISettings, Canvas: state.Canvas, SessionRecords: state.SessionRecords, Knowledge: state.Knowledge}
		state.Provider = &SQLiteProvider{store: state.SQLiteStore, repositories: state.Repositories}
		state.Harness = newSQLHarnessRepository(sqlStore)
		if err := ImportLegacyFiles(paths, git, state); err != nil {
			_ = sqlStore.Close()
			return nil, err
		}
	case StorageDriverPostgres:
		sqlStore, err := openSQLStore(config)
		if err != nil {
			return nil, err
		}
		state.SQLStore = sqlStore
		state.PostgresStore = &PostgresStore{SQLStore: state.SQLStore, url: config.DatabaseURL}
		state.Workspaces = newSQLiteWorkspaceRepository(sqlStore, paths, git)
		state.Items = &SQLiteItemRepository{db: sqlStore.db, driver: sqlStore.driver}
		state.ImportStatus = &SQLiteImportStatusRepository{db: sqlStore.db, driver: sqlStore.driver}
		state.Audit = &SQLiteAuditRepository{db: sqlStore.db, driver: sqlStore.driver, now: time.Now}
		state.Navigation = &SQLiteNavigationRepository{db: sqlStore.db, driver: sqlStore.driver, now: time.Now}
		state.AISettings = &SQLiteAISettingsRepository{db: sqlStore.db, driver: sqlStore.driver}
		state.Canvas = &SQLiteCanvasRepository{db: sqlStore.db, driver: sqlStore.driver, now: time.Now}
		state.SessionRecords = &SQLiteSessionRecordRepository{db: sqlStore.db, driver: sqlStore.driver}
		state.Cloud = &SQLCloudRepository{db: sqlStore.db, driver: sqlStore.driver, now: time.Now}
		state.Repositories = RepositoryBundle{Workspaces: state.Workspaces, Items: state.Items, Audit: state.Audit, Navigation: state.Navigation, AISettings: state.AISettings, Canvas: state.Canvas, SessionRecords: state.SessionRecords, Knowledge: state.Knowledge}
		state.Provider = &PostgresProvider{store: state.PostgresStore, repositories: state.Repositories}
		state.Harness = newSQLHarnessRepository(sqlStore)
	}
	state.StatusService = NewStorageStatusService(config, paths, runtime, state.SQLStore)
	state.SyncService = NewStorageSyncService(config, paths, runtime, git)
	return state, nil
}

func (s *SQLStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLStore) Health(ctx context.Context) DatabaseHealth {
	if s == nil || s.db == nil {
		return DatabaseHealth{}
	}
	health := DatabaseHealth{Driver: s.driver, Configured: true, OK: false, MigrationVersion: s.migrationVersion}
	if err := s.db.PingContext(ctx); err != nil {
		health.Error = err.Error()
		return health
	}
	version, err := currentMigrationVersion(ctx, s.db)
	if err != nil {
		health.Error = err.Error()
		return health
	}
	health.OK = true
	health.MigrationVersion = version
	s.migrationVersion = version
	return health
}

func openSQLStore(config Config) (*SQLStore, error) {
	driverName, dataSourceName, migrations, err := sqlOpenConfig(config)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	if config.Driver == StorageDriverSQLite {
		// One connection owns transactions in this process. The busy timeout waits for writers in other processes.
		db.SetMaxOpenConns(1)
		if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%s database unavailable: %w", config.Driver, err)
	}
	version, err := ensureMigrations(ctx, db, config.Driver, config.Migrations, migrations)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLStore{driver: config.Driver, db: db, migrationVersion: version}, nil
}

func sqlOpenConfig(config Config) (string, string, []Migration, error) {
	switch config.Driver {
	case StorageDriverSQLite:
		if strings.TrimSpace(config.SQLitePath) == "" {
			return "", "", nil, fmt.Errorf("%s=sqlite requires %s", EnvStorageDriver, EnvSQLitePath)
		}
		if err := os.MkdirAll(filepath.Dir(config.SQLitePath), 0o755); err != nil {
			return "", "", nil, err
		}
		return "sqlite", config.SQLitePath, sqliteMigrations(), nil
	case StorageDriverPostgres:
		return "pgx", config.DatabaseURL, postgresMigrations(), nil
	default:
		return "", "", nil, fmt.Errorf("%s does not support SQL migrations", config.Driver)
	}
}
