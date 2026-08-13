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
	if config.Driver == StorageDriverSQLite {
		// One connection provides deterministic transaction ownership inside a
		// process; the busy timeout coordinates short-lived writers in another
		// process instead of surfacing an immediate SQLITE_BUSY startup failure.
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
	}, {
		Version: 2,
		Name:    "canvas_and_session_records",
		SQL: `CREATE TABLE IF NOT EXISTS canvas_layouts (
id TEXT PRIMARY KEY,
owner_user_id TEXT NOT NULL DEFAULT '',
workspace_id TEXT NOT NULL,
branch_key TEXT NOT NULL,
viewport_json TEXT NOT NULL,
version INTEGER NOT NULL,
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS canvas_layouts_owner_workspace_branch ON canvas_layouts (owner_user_id, workspace_id, branch_key);
CREATE TABLE IF NOT EXISTS canvas_placements (
layout_id TEXT NOT NULL,
node_id TEXT NOT NULL,
entity_ref_json TEXT NOT NULL,
x REAL NOT NULL,
y REAL NOT NULL,
collapsed INTEGER NOT NULL DEFAULT 0,
revision INTEGER NOT NULL,
updated_at TEXT NOT NULL,
PRIMARY KEY (layout_id, node_id)
);
CREATE TABLE IF NOT EXISTS ai_session_records (
id TEXT PRIMARY KEY,
workspace_id TEXT NOT NULL,
plan_ref_json TEXT NOT NULL DEFAULT '',
provider TEXT NOT NULL,
intent TEXT NOT NULL,
requested_branch TEXT NOT NULL,
observed_commit TEXT NOT NULL DEFAULT '',
idempotency_key TEXT NOT NULL DEFAULT '',
state TEXT NOT NULL,
started_at TEXT NOT NULL,
ended_at TEXT,
exit_code INTEGER,
last_known_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS ai_session_records_workspace_idempotency ON ai_session_records (workspace_id, idempotency_key) WHERE idempotency_key <> '';`,
	}, {
		Version: 3,
		Name:    "hidden_canvas_placements",
		SQL:     `ALTER TABLE canvas_placements ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0;`,
	}, {
		Version: 4,
		Name:    "collapse_existing_canvas_sessions",
		SQL:     `UPDATE canvas_placements SET collapsed = TRUE WHERE entity_ref_json LIKE '%"kind":"session"%';`,
	}, {
		Version: 5,
		Name:    "ownership_and_cloud_state",
		SQL: `ALTER TABLE audit_events ADD COLUMN owner_user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_events ADD COLUMN actor_user_id TEXT NOT NULL DEFAULT '';
UPDATE audit_events SET owner_user_id = 'local' WHERE owner_user_id = '';
UPDATE audit_events SET actor_user_id = 'unknown' WHERE actor_user_id = '';
ALTER TABLE saved_filters ADD COLUMN owner_user_id TEXT NOT NULL DEFAULT 'local';
ALTER TABLE recent_items ADD COLUMN owner_user_id TEXT NOT NULL DEFAULT 'local';
CREATE INDEX IF NOT EXISTS audit_events_owner_workspace_time ON audit_events (owner_user_id, workspace_id, event_time DESC);
CREATE INDEX IF NOT EXISTS saved_filters_owner_updated ON saved_filters (owner_user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS recent_items_owner_opened ON recent_items (owner_user_id, opened_at DESC);
CREATE TABLE IF NOT EXISTS cloud_workspaces (owner_user_id TEXT NOT NULL, id TEXT NOT NULL, workspace_json TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY (owner_user_id, id));
CREATE TABLE IF NOT EXISTS cloud_agents (owner_user_id TEXT NOT NULL, id TEXT NOT NULL, agent_json TEXT NOT NULL, last_seen_at TEXT NOT NULL, PRIMARY KEY (owner_user_id, id));
CREATE TABLE IF NOT EXISTS cloud_provider_instances (owner_user_id TEXT NOT NULL, id TEXT NOT NULL, instance_json TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY (owner_user_id, id));
CREATE TABLE IF NOT EXISTS cloud_provider_connections (owner_user_id TEXT NOT NULL, provider_instance_id TEXT NOT NULL, encrypted_credentials TEXT NOT NULL, connection_json TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY (owner_user_id, provider_instance_id));`,
	}, {
		Version: 6,
		Name:    "owner_scoped_navigation_keys",
		SQL: `ALTER TABLE saved_filters RENAME TO saved_filters_legacy;
CREATE TABLE saved_filters (owner_user_id TEXT NOT NULL, id TEXT NOT NULL, name TEXT NOT NULL, route TEXT NOT NULL, workspace_id TEXT, filters_json TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY (owner_user_id, id));
INSERT INTO saved_filters (owner_user_id, id, name, route, workspace_id, filters_json, created_at, updated_at) SELECT owner_user_id, id, name, route, workspace_id, filters_json, created_at, updated_at FROM saved_filters_legacy;
DROP TABLE saved_filters_legacy;
CREATE INDEX saved_filters_owner_updated ON saved_filters (owner_user_id, updated_at DESC);
ALTER TABLE recent_items RENAME TO recent_items_legacy;
CREATE TABLE recent_items (owner_user_id TEXT NOT NULL, item_id TEXT NOT NULL, workspace_id TEXT NOT NULL, title TEXT NOT NULL, subtitle TEXT, route TEXT NOT NULL, opened_at TEXT NOT NULL, PRIMARY KEY (owner_user_id, item_id));
CREATE TABLE pruned_recent_items_v6 (owner_user_id TEXT NOT NULL, item_id TEXT NOT NULL, workspace_id TEXT NOT NULL, title TEXT NOT NULL, subtitle TEXT, route TEXT NOT NULL, opened_at TEXT NOT NULL);
INSERT INTO pruned_recent_items_v6 SELECT owner_user_id, item_id, workspace_id, title, subtitle, route, opened_at FROM (SELECT *, ROW_NUMBER() OVER (PARTITION BY owner_user_id ORDER BY opened_at DESC, item_id ASC) AS retention_rank FROM recent_items_legacy) WHERE retention_rank > 50;
INSERT INTO recent_items (owner_user_id, item_id, workspace_id, title, subtitle, route, opened_at) SELECT owner_user_id, item_id, workspace_id, title, subtitle, route, opened_at FROM (SELECT *, ROW_NUMBER() OVER (PARTITION BY owner_user_id ORDER BY opened_at DESC, item_id ASC) AS retention_rank FROM recent_items_legacy) WHERE retention_rank <= 50;
DROP TABLE recent_items_legacy;
CREATE INDEX recent_items_owner_opened ON recent_items (owner_user_id, opened_at DESC);
CREATE TABLE IF NOT EXISTS migration_repairs (migration_version INTEGER NOT NULL, repair_name TEXT NOT NULL, affected_rows INTEGER NOT NULL, repaired_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (migration_version, repair_name));
INSERT INTO migration_repairs (migration_version, repair_name, affected_rows) SELECT 6, 'recent_items_retention', COUNT(*) FROM pruned_recent_items_v6;`,
	}, {
		Version: 7,
		Name:    "repair_orphan_workspace_projections",
		SQL: `CREATE TABLE IF NOT EXISTS migration_repairs (migration_version INTEGER NOT NULL, repair_name TEXT NOT NULL, affected_rows INTEGER NOT NULL, repaired_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (migration_version, repair_name));
CREATE TABLE IF NOT EXISTS orphaned_indexed_items_v7 (id TEXT NOT NULL, workspace_id TEXT NOT NULL, branch TEXT NOT NULL, scope TEXT NOT NULL, identifier TEXT NOT NULL, title TEXT NOT NULL, status TEXT NOT NULL, item_path TEXT NOT NULL, source_mode TEXT NOT NULL, editable INTEGER NOT NULL, metadata_json TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS orphaned_branch_scans_v7 (workspace_id TEXT NOT NULL, branch TEXT NOT NULL, branch_ref TEXT, commit_sha TEXT, source_mode TEXT NOT NULL, editable INTEGER NOT NULL, source_configuration_hash TEXT, working_tree_hash TEXT, scanned_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS orphaned_scan_warnings_v7 (workspace_id TEXT NOT NULL, branch TEXT NOT NULL, item_path TEXT NOT NULL, code TEXT NOT NULL, message TEXT NOT NULL);
INSERT INTO migration_repairs (migration_version, repair_name, affected_rows) SELECT 7, 'indexed_items', COUNT(*) FROM indexed_items WHERE workspace_id NOT IN (SELECT id FROM workspaces);
INSERT INTO orphaned_indexed_items_v7 SELECT * FROM indexed_items WHERE workspace_id NOT IN (SELECT id FROM workspaces);
DELETE FROM indexed_items WHERE workspace_id NOT IN (SELECT id FROM workspaces);
INSERT INTO migration_repairs (migration_version, repair_name, affected_rows) SELECT 7, 'branch_scans', COUNT(*) FROM branch_scans WHERE workspace_id NOT IN (SELECT id FROM workspaces);
INSERT INTO orphaned_branch_scans_v7 SELECT * FROM branch_scans WHERE workspace_id NOT IN (SELECT id FROM workspaces);
DELETE FROM branch_scans WHERE workspace_id NOT IN (SELECT id FROM workspaces);
INSERT INTO migration_repairs (migration_version, repair_name, affected_rows) SELECT 7, 'scan_warnings', COUNT(*) FROM scan_warnings WHERE workspace_id NOT IN (SELECT id FROM workspaces);
INSERT INTO orphaned_scan_warnings_v7 SELECT * FROM scan_warnings WHERE workspace_id NOT IN (SELECT id FROM workspaces);
DELETE FROM scan_warnings WHERE workspace_id NOT IN (SELECT id FROM workspaces);`,
	}, {
		Version: 8,
		Name:    "legacy_import_coordination",
		SQL:     `CREATE TABLE IF NOT EXISTS import_locks (source_name TEXT PRIMARY KEY, acquired_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);`,
	}, {
		Version: 9,
		Name:    "cloud_agent_enrollment_tokens",
		SQL:     `CREATE TABLE IF NOT EXISTS cloud_agent_enrollment_tokens (id TEXT PRIMARY KEY, expires_at TEXT NOT NULL, consumed_at TEXT NOT NULL);`,
	}, {
		Version: 10,
		Name:    "cloud_workspace_publication_revisions",
		SQL:     `ALTER TABLE cloud_workspaces ADD COLUMN publication_revision INTEGER NOT NULL DEFAULT 0;`,
	}}
}

func postgresMigrations() []Migration {
	return []Migration{{Version: 1, Name: "app_owned_state", SQL: postgresAppOwnedStateDDL},
		{Version: 2, Name: "canvas_and_session_records", SQL: postgresCanvasAndSessionsDDL},
		{Version: 3, Name: "hidden_canvas_placements", SQL: `ALTER TABLE canvas_placements ADD COLUMN hidden BOOLEAN NOT NULL DEFAULT FALSE;`},
		{Version: 4, Name: "collapse_existing_canvas_sessions", SQL: `UPDATE canvas_placements SET collapsed = TRUE WHERE entity_ref_json LIKE '%"kind":"session"%';`},
		{Version: 5, Name: "ownership_and_cloud_state", SQL: postgresOwnershipAndCloudDDL},
		{Version: 6, Name: "owner_scoped_navigation_keys", SQL: postgresOwnerScopedNavigationDDL},
		{Version: 7, Name: "repair_orphan_workspace_projections", SQL: repairOrphanWorkspaceProjectionsDDL},
		{Version: 8, Name: "legacy_import_coordination", SQL: `CREATE TABLE IF NOT EXISTS import_locks (source_name TEXT PRIMARY KEY, acquired_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP);`},
		{Version: 9, Name: "cloud_agent_enrollment_tokens", SQL: `CREATE TABLE IF NOT EXISTS cloud_agent_enrollment_tokens (id TEXT PRIMARY KEY, expires_at TIMESTAMPTZ NOT NULL, consumed_at TIMESTAMPTZ NOT NULL);`},
		{Version: 10, Name: "cloud_workspace_publication_revisions", SQL: `ALTER TABLE cloud_workspaces ADD COLUMN IF NOT EXISTS publication_revision BIGINT NOT NULL DEFAULT 0;`}}

}

const postgresAppOwnedStateDDL = `CREATE TABLE IF NOT EXISTS workspaces (
id TEXT PRIMARY KEY, name TEXT NOT NULL, path_label TEXT NOT NULL, baseline_branch TEXT NOT NULL,
registration_mode TEXT NOT NULL, remote_url TEXT, clone_path_managed BOOLEAN NOT NULL DEFAULT FALSE,
last_selected_branch TEXT, sources_json TEXT NOT NULL, runtime_json TEXT, workspace_json TEXT NOT NULL,
created_at TIMESTAMPTZ NOT NULL, last_scanned_at TIMESTAMPTZ);
CREATE TABLE IF NOT EXISTS branch_scans (workspace_id TEXT NOT NULL, branch TEXT NOT NULL, branch_ref TEXT, commit_sha TEXT, source_mode TEXT NOT NULL, editable BOOLEAN NOT NULL, source_configuration_hash TEXT, working_tree_hash TEXT, scanned_at TIMESTAMPTZ NOT NULL, PRIMARY KEY (workspace_id, branch));
CREATE TABLE IF NOT EXISTS indexed_items (id TEXT NOT NULL, workspace_id TEXT NOT NULL, branch TEXT NOT NULL, scope TEXT NOT NULL, identifier TEXT NOT NULL, title TEXT NOT NULL, status TEXT NOT NULL, item_path TEXT NOT NULL, source_mode TEXT NOT NULL, editable BOOLEAN NOT NULL, metadata_json TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL, PRIMARY KEY (workspace_id, branch, id));
CREATE INDEX IF NOT EXISTS indexed_items_workspace_branch_status ON indexed_items (workspace_id, branch, status);
CREATE TABLE IF NOT EXISTS scan_warnings (workspace_id TEXT NOT NULL, branch TEXT NOT NULL, item_path TEXT NOT NULL, code TEXT NOT NULL, message TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS audit_events (id TEXT PRIMARY KEY, workspace_id TEXT, item_id TEXT, operation TEXT NOT NULL, status TEXT NOT NULL, message TEXT NOT NULL, paths_json TEXT NOT NULL, duration_ms BIGINT NOT NULL, error TEXT, event_time TIMESTAMPTZ NOT NULL);
CREATE TABLE IF NOT EXISTS saved_filters (id TEXT PRIMARY KEY, name TEXT NOT NULL, route TEXT NOT NULL, workspace_id TEXT, filters_json TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL);
CREATE TABLE IF NOT EXISTS recent_items (item_id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, title TEXT NOT NULL, subtitle TEXT, route TEXT NOT NULL, opened_at TIMESTAMPTZ NOT NULL);
CREATE TABLE IF NOT EXISTS ai_settings (id TEXT PRIMARY KEY, settings_json TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL);
CREATE TABLE IF NOT EXISTS knowledge_indexes (workspace_id TEXT NOT NULL, root TEXT NOT NULL, wiki_json TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL, PRIMARY KEY (workspace_id, root));
CREATE TABLE IF NOT EXISTS import_status (source_name TEXT PRIMARY KEY, completed_at TIMESTAMPTZ NOT NULL);`

const postgresCanvasAndSessionsDDL = `CREATE TABLE IF NOT EXISTS canvas_layouts (id TEXT PRIMARY KEY, owner_user_id TEXT NOT NULL DEFAULT '', workspace_id TEXT NOT NULL, branch_key TEXT NOT NULL, viewport_json TEXT NOT NULL, version INTEGER NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL);
CREATE UNIQUE INDEX IF NOT EXISTS canvas_layouts_owner_workspace_branch ON canvas_layouts (owner_user_id, workspace_id, branch_key);
CREATE TABLE IF NOT EXISTS canvas_placements (layout_id TEXT NOT NULL, node_id TEXT NOT NULL, entity_ref_json TEXT NOT NULL, x DOUBLE PRECISION NOT NULL, y DOUBLE PRECISION NOT NULL, collapsed BOOLEAN NOT NULL DEFAULT FALSE, revision INTEGER NOT NULL, updated_at TIMESTAMPTZ NOT NULL, PRIMARY KEY (layout_id, node_id));
CREATE TABLE IF NOT EXISTS ai_session_records (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, plan_ref_json TEXT NOT NULL DEFAULT '', provider TEXT NOT NULL, intent TEXT NOT NULL, requested_branch TEXT NOT NULL, observed_commit TEXT NOT NULL DEFAULT '', idempotency_key TEXT NOT NULL DEFAULT '', state TEXT NOT NULL, started_at TIMESTAMPTZ NOT NULL, ended_at TIMESTAMPTZ, exit_code INTEGER, last_known_at TIMESTAMPTZ NOT NULL);
CREATE UNIQUE INDEX IF NOT EXISTS ai_session_records_workspace_idempotency ON ai_session_records (workspace_id, idempotency_key) WHERE idempotency_key <> '';`

const postgresOwnershipAndCloudDDL = `ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS owner_user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS actor_user_id TEXT NOT NULL DEFAULT '';
UPDATE audit_events SET owner_user_id = 'local' WHERE owner_user_id = '';
UPDATE audit_events SET actor_user_id = 'unknown' WHERE actor_user_id = '';
DO $$ BEGIN
IF (SELECT data_type IN ('integer','bigint','smallint') FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'workspaces' AND column_name = 'clone_path_managed') THEN ALTER TABLE workspaces ALTER COLUMN clone_path_managed TYPE BOOLEAN USING clone_path_managed <> 0; END IF;
IF (SELECT data_type IN ('integer','bigint','smallint') FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'branch_scans' AND column_name = 'editable') THEN ALTER TABLE branch_scans ALTER COLUMN editable TYPE BOOLEAN USING editable <> 0; END IF;
IF (SELECT data_type IN ('integer','bigint','smallint') FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'indexed_items' AND column_name = 'editable') THEN ALTER TABLE indexed_items ALTER COLUMN editable TYPE BOOLEAN USING editable <> 0; END IF;
IF (SELECT data_type IN ('integer','bigint','smallint') FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'canvas_placements' AND column_name = 'collapsed') THEN ALTER TABLE canvas_placements ALTER COLUMN collapsed TYPE BOOLEAN USING collapsed <> 0; END IF;
IF (SELECT data_type IN ('integer','bigint','smallint') FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'canvas_placements' AND column_name = 'hidden') THEN ALTER TABLE canvas_placements ALTER COLUMN hidden TYPE BOOLEAN USING hidden <> 0; END IF;
END $$;
ALTER TABLE workspaces ALTER COLUMN clone_path_managed SET DEFAULT FALSE;
ALTER TABLE canvas_placements ALTER COLUMN collapsed SET DEFAULT FALSE;
ALTER TABLE canvas_placements ALTER COLUMN hidden SET DEFAULT FALSE;
ALTER TABLE saved_filters ADD COLUMN IF NOT EXISTS owner_user_id TEXT NOT NULL DEFAULT 'local';
ALTER TABLE recent_items ADD COLUMN IF NOT EXISTS owner_user_id TEXT NOT NULL DEFAULT 'local';
CREATE INDEX IF NOT EXISTS audit_events_owner_workspace_time ON audit_events (owner_user_id, workspace_id, event_time DESC);
CREATE INDEX IF NOT EXISTS saved_filters_owner_updated ON saved_filters (owner_user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS recent_items_owner_opened ON recent_items (owner_user_id, opened_at DESC);
CREATE TABLE IF NOT EXISTS cloud_workspaces (owner_user_id TEXT NOT NULL, id TEXT NOT NULL, workspace_json TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL, PRIMARY KEY (owner_user_id, id));
CREATE TABLE IF NOT EXISTS cloud_agents (owner_user_id TEXT NOT NULL, id TEXT NOT NULL, agent_json TEXT NOT NULL, last_seen_at TIMESTAMPTZ NOT NULL, PRIMARY KEY (owner_user_id, id));
CREATE TABLE IF NOT EXISTS cloud_provider_instances (owner_user_id TEXT NOT NULL, id TEXT NOT NULL, instance_json TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL, PRIMARY KEY (owner_user_id, id));
CREATE TABLE IF NOT EXISTS cloud_provider_connections (owner_user_id TEXT NOT NULL, provider_instance_id TEXT NOT NULL, encrypted_credentials TEXT NOT NULL, connection_json TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL, PRIMARY KEY (owner_user_id, provider_instance_id));`

const postgresOwnerScopedNavigationDDL = `ALTER TABLE saved_filters DROP CONSTRAINT IF EXISTS saved_filters_pkey;
ALTER TABLE saved_filters ADD PRIMARY KEY (owner_user_id, id);
ALTER TABLE recent_items DROP CONSTRAINT IF EXISTS recent_items_pkey;
ALTER TABLE recent_items ADD PRIMARY KEY (owner_user_id, item_id);
CREATE TABLE IF NOT EXISTS pruned_recent_items_v6 (owner_user_id TEXT NOT NULL, item_id TEXT NOT NULL, workspace_id TEXT NOT NULL, title TEXT NOT NULL, subtitle TEXT, route TEXT NOT NULL, opened_at TIMESTAMPTZ NOT NULL);
INSERT INTO pruned_recent_items_v6 SELECT owner_user_id, item_id, workspace_id, title, subtitle, route, opened_at FROM (SELECT *, ROW_NUMBER() OVER (PARTITION BY owner_user_id ORDER BY opened_at DESC, item_id ASC) AS retention_rank FROM recent_items) ranked WHERE retention_rank > 50;
DELETE FROM recent_items WHERE (owner_user_id, item_id) IN (SELECT owner_user_id, item_id FROM pruned_recent_items_v6);
CREATE TABLE IF NOT EXISTS migration_repairs (migration_version INTEGER NOT NULL, repair_name TEXT NOT NULL, affected_rows INTEGER NOT NULL, repaired_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (migration_version, repair_name));
INSERT INTO migration_repairs (migration_version, repair_name, affected_rows) SELECT 6, 'recent_items_retention', COUNT(*) FROM pruned_recent_items_v6;`

const repairOrphanWorkspaceProjectionsDDL = `CREATE TABLE IF NOT EXISTS migration_repairs (migration_version INTEGER NOT NULL, repair_name TEXT NOT NULL, affected_rows INTEGER NOT NULL, repaired_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (migration_version, repair_name));
CREATE TABLE IF NOT EXISTS orphaned_indexed_items_v7 (id TEXT NOT NULL, workspace_id TEXT NOT NULL, branch TEXT NOT NULL, scope TEXT NOT NULL, identifier TEXT NOT NULL, title TEXT NOT NULL, status TEXT NOT NULL, item_path TEXT NOT NULL, source_mode TEXT NOT NULL, editable BOOLEAN NOT NULL, metadata_json TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL);
CREATE TABLE IF NOT EXISTS orphaned_branch_scans_v7 (workspace_id TEXT NOT NULL, branch TEXT NOT NULL, branch_ref TEXT, commit_sha TEXT, source_mode TEXT NOT NULL, editable BOOLEAN NOT NULL, source_configuration_hash TEXT, working_tree_hash TEXT, scanned_at TIMESTAMPTZ NOT NULL);
CREATE TABLE IF NOT EXISTS orphaned_scan_warnings_v7 (workspace_id TEXT NOT NULL, branch TEXT NOT NULL, item_path TEXT NOT NULL, code TEXT NOT NULL, message TEXT NOT NULL);
INSERT INTO migration_repairs (migration_version, repair_name, affected_rows) SELECT 7, 'indexed_items', COUNT(*) FROM indexed_items WHERE workspace_id NOT IN (SELECT id FROM workspaces);
INSERT INTO orphaned_indexed_items_v7 SELECT * FROM indexed_items WHERE workspace_id NOT IN (SELECT id FROM workspaces);
DELETE FROM indexed_items WHERE workspace_id NOT IN (SELECT id FROM workspaces);
INSERT INTO migration_repairs (migration_version, repair_name, affected_rows) SELECT 7, 'branch_scans', COUNT(*) FROM branch_scans WHERE workspace_id NOT IN (SELECT id FROM workspaces);
INSERT INTO orphaned_branch_scans_v7 SELECT * FROM branch_scans WHERE workspace_id NOT IN (SELECT id FROM workspaces);
DELETE FROM branch_scans WHERE workspace_id NOT IN (SELECT id FROM workspaces);
INSERT INTO migration_repairs (migration_version, repair_name, affected_rows) SELECT 7, 'scan_warnings', COUNT(*) FROM scan_warnings WHERE workspace_id NOT IN (SELECT id FROM workspaces);
INSERT INTO orphaned_scan_warnings_v7 SELECT * FROM scan_warnings WHERE workspace_id NOT IN (SELECT id FROM workspaces);
DELETE FROM scan_warnings WHERE workspace_id NOT IN (SELECT id FROM workspaces);`
