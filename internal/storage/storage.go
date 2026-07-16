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

type Migration struct {
	Version int
	Name    string
	SQL     string
}

type DatabaseHealth struct {
	Driver           string `json:"driver"`
	OK               bool   `json:"ok"`
	MigrationVersion int    `json:"migrationVersion"`
	Error            string `json:"error,omitempty"`
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
		sqlStore, err := openSQLStore(config)
		if err != nil {
			return nil, err
		}
		state.SQLStore = sqlStore
		state.SQLiteStore = &SQLiteStore{SQLStore: state.SQLStore, path: config.SQLitePath}
		state.Workspaces = newSQLiteWorkspaceRepository(sqlStore, paths, git)
		state.Items = &SQLiteItemRepository{db: sqlStore.db}
		state.ImportStatus = &SQLiteImportStatusRepository{db: sqlStore.db}
		state.Audit = &SQLiteAuditRepository{db: sqlStore.db, now: time.Now}
		state.Navigation = &SQLiteNavigationRepository{db: sqlStore.db, now: time.Now}
		state.AISettings = &SQLiteAISettingsRepository{db: sqlStore.db}
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
	}
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
		return DatabaseHealth{OK: false, Error: "database is not configured"}
	}
	health := DatabaseHealth{Driver: s.driver, OK: false, MigrationVersion: s.migrationVersion}
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

func ensureMigrations(ctx context.Context, db *sql.DB, driver, mode string, migrations []Migration) (int, error) {
	if mode == "manual" {
		version, err := currentMigrationVersion(ctx, db)
		if err != nil {
			return 0, fmt.Errorf("migration required: %w", err)
		}
		if version < latestMigrationVersion(migrations) {
			return version, fmt.Errorf("migration required: current version %d, latest version %d", version, latestMigrationVersion(migrations))
		}
		return version, nil
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
version INTEGER PRIMARY KEY,
name TEXT NOT NULL,
applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`); err != nil {
		return 0, err
	}
	version, err := currentMigrationVersion(ctx, db)
	if err != nil {
		return 0, err
	}
	for _, migration := range migrations {
		if migration.Version <= version {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return version, err
		}
		if _, err = tx.ExecContext(ctx, migration.SQL); err != nil {
			_ = tx.Rollback()
			return version, fmt.Errorf("migration %d %s: %w", migration.Version, migration.Name, err)
		}
		if _, err = tx.ExecContext(ctx, insertMigrationSQL(driver), migration.Version, migration.Name); err != nil {
			_ = tx.Rollback()
			return version, err
		}
		if err = tx.Commit(); err != nil {
			return version, err
		}
		version = migration.Version
	}
	return version, nil
}

func insertMigrationSQL(driver string) string {
	if driver == StorageDriverPostgres {
		return `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`
	}
	return `INSERT INTO schema_migrations (version, name) VALUES (?, ?)`
}

func currentMigrationVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func latestMigrationVersion(migrations []Migration) int {
	latest := 0
	for _, migration := range migrations {
		if migration.Version > latest {
			latest = migration.Version
		}
	}
	return latest
}

func sqliteMigrations() []Migration {
	return []Migration{{
		Version: 1,
		Name:    "app_owned_state",
		SQL: `CREATE TABLE IF NOT EXISTS workspaces (
id TEXT PRIMARY KEY,
name TEXT NOT NULL,
path_label TEXT NOT NULL,
baseline_branch TEXT NOT NULL,
registration_mode TEXT NOT NULL,
remote_url TEXT,
clone_path_managed INTEGER NOT NULL DEFAULT 0,
last_selected_branch TEXT,
sources_json TEXT NOT NULL,
runtime_json TEXT,
workspace_json TEXT NOT NULL,
created_at TEXT NOT NULL,
last_scanned_at TEXT
);
CREATE TABLE IF NOT EXISTS branch_scans (
workspace_id TEXT NOT NULL,
branch TEXT NOT NULL,
branch_ref TEXT,
commit_sha TEXT,
source_mode TEXT NOT NULL,
editable INTEGER NOT NULL,
source_configuration_hash TEXT,
working_tree_hash TEXT,
scanned_at TEXT NOT NULL,
PRIMARY KEY (workspace_id, branch)
);
CREATE TABLE IF NOT EXISTS indexed_items (
id TEXT NOT NULL,
workspace_id TEXT NOT NULL,
branch TEXT NOT NULL,
scope TEXT NOT NULL,
identifier TEXT NOT NULL,
title TEXT NOT NULL,
status TEXT NOT NULL,
item_path TEXT NOT NULL,
source_mode TEXT NOT NULL,
editable INTEGER NOT NULL,
metadata_json TEXT NOT NULL,
updated_at TEXT NOT NULL,
PRIMARY KEY (workspace_id, branch, id)
);
CREATE INDEX IF NOT EXISTS indexed_items_workspace_branch_status ON indexed_items (workspace_id, branch, status);
CREATE TABLE IF NOT EXISTS scan_warnings (workspace_id TEXT NOT NULL, branch TEXT NOT NULL, item_path TEXT NOT NULL, code TEXT NOT NULL, message TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS audit_events (id TEXT PRIMARY KEY, workspace_id TEXT, item_id TEXT, operation TEXT NOT NULL, status TEXT NOT NULL, message TEXT NOT NULL, paths_json TEXT NOT NULL, duration_ms INTEGER NOT NULL, error TEXT, event_time TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS saved_filters (id TEXT PRIMARY KEY, name TEXT NOT NULL, route TEXT NOT NULL, workspace_id TEXT, filters_json TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS recent_items (item_id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, title TEXT NOT NULL, subtitle TEXT, route TEXT NOT NULL, opened_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS ai_settings (id TEXT PRIMARY KEY, settings_json TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS knowledge_indexes (workspace_id TEXT NOT NULL, root TEXT NOT NULL, wiki_json TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY (workspace_id, root));
CREATE TABLE IF NOT EXISTS import_status (source_name TEXT PRIMARY KEY, completed_at TEXT NOT NULL);`,
	}}
}

func postgresMigrations() []Migration {
	sqlite := sqliteMigrations()[0]
	sqlite.SQL = strings.NewReplacer(
		"INTEGER PRIMARY KEY", "INTEGER PRIMARY KEY",
		"TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP", "TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP",
		"created_at TEXT", "created_at TIMESTAMPTZ",
		"last_scanned_at TEXT", "last_scanned_at TIMESTAMPTZ",
		"scanned_at TEXT", "scanned_at TIMESTAMPTZ",
		"event_time TEXT", "event_time TIMESTAMPTZ",
		"updated_at TEXT", "updated_at TIMESTAMPTZ",
		"opened_at TEXT", "opened_at TIMESTAMPTZ",
		"completed_at TEXT", "completed_at TIMESTAMPTZ",
		"clone_path_managed INTEGER", "clone_path_managed BOOLEAN",
		"editable INTEGER", "editable BOOLEAN",
	).Replace(sqlite.SQL)
	return []Migration{sqlite}
}
