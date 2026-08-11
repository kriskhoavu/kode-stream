package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kode-stream/internal/ai"
	"kode-stream/internal/audit"
	"kode-stream/internal/canvas"
	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/navigation"
	"kode-stream/internal/system"
	"kode-stream/internal/workspace/registry"
)

func TestResolveConfigDefaultsLocalModeToDataDir(t *testing.T) {
	paths := system.Paths{SQLiteDatabaseFile: filepath.Join(t.TempDir(), "kode-stream.db")}
	config, err := ResolveConfig(system.RuntimeConfig{Mode: models.RuntimeModeLocal}, paths, emptyEnv)
	if err != nil {
		t.Fatalf("ResolveConfig returned error: %v", err)
	}
	if config.StorageOption != StorageOptionDataDir || config.Driver != StorageDriverFile {
		t.Fatalf("config = %#v, want datadir/file", config)
	}
	if config.SQLitePath != paths.SQLiteDatabaseFile {
		t.Fatalf("sqlite path = %q, want %q", config.SQLitePath, paths.SQLiteDatabaseFile)
	}
	if config.Migrations != "auto" {
		t.Fatalf("migrations = %q, want auto", config.Migrations)
	}
}

func TestResolveConfigUsesExplicitDatabaseOptionForLocalSQLite(t *testing.T) {
	paths := system.Paths{SQLiteDatabaseFile: filepath.Join(t.TempDir(), "kode-stream.db")}
	config, err := ResolveConfig(system.RuntimeConfig{Mode: models.RuntimeModeLocal}, paths, mapEnv(map[string]string{
		EnvStorageOption: StorageOptionDatabase,
	}))
	if err != nil {
		t.Fatalf("ResolveConfig returned error: %v", err)
	}
	if config.StorageOption != StorageOptionDatabase || config.Driver != StorageDriverSQLite {
		t.Fatalf("config = %#v, want database/sqlite", config)
	}
}

func TestResolveConfigRequiresPostgresInCloudMode(t *testing.T) {
	_, err := ResolveConfig(system.RuntimeConfig{Mode: models.RuntimeModeCloud}, system.Paths{}, emptyEnv)
	if err == nil {
		t.Fatal("expected missing Postgres URL error")
	}
}

func TestResolveConfigAcceptsCloudPostgresURL(t *testing.T) {
	config, err := ResolveConfig(system.RuntimeConfig{Mode: models.RuntimeModeCloud}, system.Paths{}, mapEnv(map[string]string{
		EnvDatabaseURL: "postgres://kode-stream@localhost/kode_stream",
	}))
	if err != nil {
		t.Fatalf("ResolveConfig returned error: %v", err)
	}
	if config.Driver != StorageDriverPostgres {
		t.Fatalf("driver = %q, want %q", config.Driver, StorageDriverPostgres)
	}
}

func TestResolveConfigUsesStorageOption(t *testing.T) {
	paths := system.Paths{SQLiteDatabaseFile: filepath.Join(t.TempDir(), "kode-stream.db")}
	config, err := ResolveConfig(system.RuntimeConfig{Mode: models.RuntimeModeLocal}, paths, mapEnv(map[string]string{
		EnvStorageOption: StorageOptionDataDir,
	}))
	if err != nil {
		t.Fatalf("ResolveConfig returned error: %v", err)
	}
	if config.StorageOption != StorageOptionDataDir || config.Driver != StorageDriverFile {
		t.Fatalf("config = %#v, want datadir/file", config)
	}
}

func TestResolveConfigRejectsCloudDataDir(t *testing.T) {
	_, err := ResolveConfig(system.RuntimeConfig{Mode: models.RuntimeModeCloud}, system.Paths{}, mapEnv(map[string]string{
		EnvStorageOption: StorageOptionDataDir,
	}))
	if err == nil {
		t.Fatal("expected cloud datadir validation error")
	}
}

func TestOpenAppOwnedStateRunsSQLiteMigrations(t *testing.T) {
	dataDir := t.TempDir()
	paths := system.Paths{
		Dir:                dataDir,
		RegistryFile:       filepath.Join(dataDir, "workspaces.yaml"),
		PlanIndexFile:      filepath.Join(dataDir, "item-index.yaml"),
		SQLiteDatabaseFile: filepath.Join(dataDir, "kode-stream.db"),
		KnowledgeIndexFile: filepath.Join(dataDir, "knowledge-index.yaml"),
		AuditLogFile:       filepath.Join(dataDir, "audit-log.jsonl"),
		SavedFiltersFile:   filepath.Join(dataDir, "saved-filters.yaml"),
		RecentItemsFile:    filepath.Join(dataDir, "recent-items.yaml"),
		AISettingsFile:     filepath.Join(dataDir, "ai-settings.yaml"),
	}
	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil, databaseEnv)
	if err != nil {
		t.Fatalf("OpenAppOwnedState returned error: %v", err)
	}
	defer state.SQLStore.Close()
	health := state.SQLStore.Health(context.Background())
	if !health.OK {
		t.Fatalf("health = %#v", health)
	}
	if health.Driver != StorageDriverSQLite || health.MigrationVersion != 8 {
		t.Fatalf("health = %#v, want sqlite version 8", health)
	}
	for _, table := range []string{"workspaces", "branch_scans", "indexed_items", "import_status", "canvas_layouts", "canvas_placements", "ai_session_records"} {
		var name string
		err := state.SQLStore.db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s was not created: %v", table, err)
		}
	}
	var hiddenColumn string
	if err := state.SQLStore.db.QueryRow(`SELECT name FROM pragma_table_info('canvas_placements') WHERE name = 'hidden'`).Scan(&hiddenColumn); err != nil || hiddenColumn != "hidden" {
		t.Fatalf("canvas placement hidden column was not created: %q %v", hiddenColumn, err)
	}
}

func TestCanvasSessionDisclosureMigrationIsCrossDriverSafe(t *testing.T) {
	sqlite := sqliteMigrations()
	postgres := postgresMigrations()
	if sqlite[3].Version != 4 || postgres[3].Version != 4 {
		t.Fatalf("session disclosure migration version diverged: sqlite %d postgres %d", sqlite[3].Version, postgres[3].Version)
	}
	if sqlite[3].SQL != postgres[3].SQL {
		t.Fatalf("session disclosure migration diverged across drivers: sqlite=%q postgres=%q", sqlite[3].SQL, postgres[3].SQL)
	}
}

func TestSQLiteOwnershipMigrationsUpgradeExistingSchemaWithoutReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrations := sqliteMigrations()
	if _, err := ensureMigrations(context.Background(), db, StorageDriverSQLite, "auto", migrations[:4]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO saved_filters (id, name, route, filters_json, created_at, updated_at) VALUES ('legacy-filter', 'Legacy', '/workstream', '{}', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 51; index++ {
		if _, err := db.Exec(`INSERT INTO recent_items (item_id, workspace_id, title, route, opened_at) VALUES (?, 'workspace', 'Recent', '/items/recent', ?)`, fmt.Sprintf("recent-%02d", index), fmt.Sprintf("2026-01-01T00:%02d:00Z", index)); err != nil {
			t.Fatal(err)
		}
	}
	if version, err := ensureMigrations(context.Background(), db, StorageDriverSQLite, "auto", migrations); err != nil || version != 8 {
		t.Fatalf("upgrade version=%d err=%v", version, err)
	}
	var owner string
	if err := db.QueryRow(`SELECT owner_user_id FROM saved_filters WHERE id = 'legacy-filter'`).Scan(&owner); err != nil || owner != navigation.LocalOwner {
		t.Fatalf("legacy owner=%q err=%v", owner, err)
	}
	if _, err := db.Exec(`INSERT INTO saved_filters (owner_user_id, id, name, route, filters_json, created_at, updated_at) VALUES ('other-owner', 'legacy-filter', 'Other', '/workstream', '{}', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("composite owner key not installed: %v", err)
	}
	var retained, quarantined, reported int
	if err := db.QueryRow(`SELECT COUNT(*) FROM recent_items WHERE owner_user_id = ?`, navigation.LocalOwner).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM pruned_recent_items_v6 WHERE owner_user_id = ?`, navigation.LocalOwner).Scan(&quarantined); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT affected_rows FROM migration_repairs WHERE migration_version = 6 AND repair_name = 'recent_items_retention'`).Scan(&reported); err != nil {
		t.Fatal(err)
	}
	if retained != 50 || quarantined != 1 || reported != 1 {
		t.Fatalf("recent retention retained=%d quarantined=%d reported=%d", retained, quarantined, reported)
	}
}

func TestOrphanRepairMigrationQuarantinesRowsBeforeDeletion(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "orphans.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrations := sqliteMigrations()
	if _, err := ensureMigrations(context.Background(), db, StorageDriverSQLite, "auto", migrations[:6]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO indexed_items (id, workspace_id, branch, scope, identifier, title, status, item_path, source_mode, editable, metadata_json, updated_at) VALUES ('item', 'missing', 'main', 'plans', 'PM-1', 'Lost', 'draft', 'plans/PM-1', 'working_tree', 1, '{"preserve":true}', '2026-01-01T00:00:00Z');
INSERT INTO branch_scans (workspace_id, branch, source_mode, editable, scanned_at) VALUES ('missing', 'main', 'working_tree', 1, '2026-01-01T00:00:00Z');
INSERT INTO scan_warnings (workspace_id, branch, item_path, code, message) VALUES ('missing', 'main', 'plans/PM-1', 'warning', 'preserve me')`); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureMigrations(context.Background(), db, StorageDriverSQLite, "auto", migrations); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct{ table, value string }{
		{"orphaned_indexed_items_v7", "item"},
		{"orphaned_branch_scans_v7", "missing"},
		{"orphaned_scan_warnings_v7", "preserve me"},
	} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM "+check.table+" WHERE id = ?", check.value).Scan(&count); err != nil {
			// The quarantine tables have domain-specific evidence columns.
			query := map[string]string{
				"orphaned_branch_scans_v7":  "SELECT COUNT(*) FROM orphaned_branch_scans_v7 WHERE workspace_id = ?",
				"orphaned_scan_warnings_v7": "SELECT COUNT(*) FROM orphaned_scan_warnings_v7 WHERE message = ?",
			}[check.table]
			if query == "" || db.QueryRow(query, check.value).Scan(&count) != nil {
				t.Fatalf("query quarantine %s: %v", check.table, err)
			}
		}
		if count != 1 {
			t.Fatalf("quarantine %s count = %d", check.table, count)
		}
	}
	for _, table := range []string{"indexed_items", "branch_scans", "scan_warnings"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE workspace_id = 'missing'").Scan(&count); err != nil || count != 0 {
			t.Fatalf("active table %s count=%d err=%v", table, count, err)
		}
	}
}

func TestPostgresMigrationsUseExplicitBooleanDDL(t *testing.T) {
	ddl := postgresMigrations()
	for _, migration := range ddl {
		if strings.Contains(migration.SQL, "BOOLEAN NOT NULL DEFAULT 0") {
			t.Fatalf("migration %d uses SQLite Boolean default", migration.Version)
		}
	}
	if !strings.Contains(postgresOwnershipAndCloudDDL, "TYPE BOOLEAN USING") || !strings.Contains(postgresOwnershipAndCloudDDL, "SET DEFAULT FALSE") {
		t.Fatal("existing Postgres Boolean repair is missing")
	}
}

func TestOpenAppOwnedStateRequiresManualMigrationsToExist(t *testing.T) {
	dataDir := t.TempDir()
	paths := system.Paths{
		SQLiteDatabaseFile: filepath.Join(dataDir, "kode-stream.db"),
		RegistryFile:       filepath.Join(dataDir, "workspaces.yaml"),
		PlanIndexFile:      filepath.Join(dataDir, "item-index.yaml"),
		AuditLogFile:       filepath.Join(dataDir, "audit-log.jsonl"),
		SavedFiltersFile:   filepath.Join(dataDir, "saved-filters.yaml"),
		RecentItemsFile:    filepath.Join(dataDir, "recent-items.yaml"),
		AISettingsFile:     filepath.Join(dataDir, "ai-settings.yaml"),
		KnowledgeIndexFile: filepath.Join(dataDir, "knowledge-index.yaml"),
	}
	_, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil, mapEnv(map[string]string{
		EnvStorageOption: StorageOptionDatabase,
		EnvMigrations:    "manual",
	}))
	if err == nil {
		t.Fatal("expected manual migration error")
	}
}

func TestOpenAppOwnedStateImportsLegacyAppOwnedFiles(t *testing.T) {
	dataDir := t.TempDir()
	workspaceRoot := t.TempDir()
	runGit(t, workspaceRoot, "init")
	runGit(t, workspaceRoot, "config", "user.email", "test@example.com")
	runGit(t, workspaceRoot, "config", "user.name", "Test User")
	if err := os.Mkdir(filepath.Join(workspaceRoot, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "plans", "README.md"), []byte("# item"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, workspaceRoot, "add", ".")
	runGit(t, workspaceRoot, "commit", "-m", "initial")

	paths := testPaths(dataDir)
	git := gitadapter.New()
	legacyRegistry := registry.New(paths.RegistryFile, git)
	workspace, err := legacyRegistry.Create(models.WorkspaceInput{Name: "Workspace", Path: workspaceRoot, BaselineBranch: "master", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "item-1", WorkspaceID: workspace.ID, WorkspaceName: workspace.Name, Branch: "master", Scope: "plans", Identifier: "README", Title: "Readme", Status: models.StatusDraft, ItemPath: "plans", SourceMode: "working_tree", Editable: true, UpdatedAt: time.Now().UTC()}}
	if err := itemindex.New(paths.PlanIndexFile).ReplaceWorkspaceBranch(workspace.ID, "master", []models.ItemDetail{item}, models.BranchScanMetadata{WorkspaceID: workspace.ID, Branch: "master", SourceMode: "working_tree", Editable: true, ScannedAt: time.Now().UTC(), Warnings: []models.ScanWarning{{ItemPath: "plans", Message: "warning"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := audit.New(paths.AuditLogFile).Append(models.AuditEvent{WorkspaceID: workspace.ID, Operation: "scan", Status: models.AuditStatusSuccess, Message: "ok"}); err != nil {
		t.Fatal(err)
	}
	nav := navigation.New(paths.SavedFiltersFile, paths.RecentItemsFile)
	if _, err := nav.SaveFilter(models.SavedFilter{Name: "Drafts", Route: "/items", Filters: map[string]any{"status": "draft"}}); err != nil {
		t.Fatal(err)
	}
	if err := nav.RecordRecent(models.RecentItem{ItemID: item.ID, WorkspaceID: workspace.ID, Title: item.Title, Route: "/items/item-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ai.NewSettingsRepository(paths.AISettingsFile).Save(ai.Settings{DefaultProvider: "codex", Providers: map[string]ai.LaunchTemplate{"codex": {Enabled: true, Executable: "codex"}}, Terminals: map[string]ai.LaunchTemplate{}}); err != nil {
		t.Fatal(err)
	}

	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, git, databaseEnv)
	if err != nil {
		t.Fatalf("OpenAppOwnedState returned error: %v", err)
	}
	defer state.SQLStore.Close()
	workspaces, err := state.Workspaces.List()
	if err != nil || len(workspaces) != 1 {
		t.Fatalf("workspaces = %#v, err = %v", workspaces, err)
	}
	items, err := state.Items.BranchItems(workspace.ID, "master")
	if err != nil || len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("items = %#v, err = %v", items, err)
	}
	metadata, ok, err := state.Items.BranchScan(workspace.ID, "master")
	if err != nil || !ok || len(metadata.Warnings) != 1 {
		t.Fatalf("metadata = %#v ok=%v err=%v", metadata, ok, err)
	}
	events, err := state.Audit.Recent(10)
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %#v, err = %v", events, err)
	}
	filters, err := state.Navigation.Filters()
	if err != nil || len(filters) != 1 {
		t.Fatalf("filters = %#v, err = %v", filters, err)
	}
	settings, err := state.AISettings.Load()
	if err != nil || settings.DefaultProvider != "codex" {
		t.Fatalf("settings = %#v, err = %v", settings, err)
	}
	if _, err := os.Stat(paths.RegistryFile); err != nil {
		t.Fatalf("legacy source file was changed or removed: %v", err)
	}
}

func TestOpenAppOwnedStateUsesDataDirProvider(t *testing.T) {
	dataDir := t.TempDir()
	paths := testPaths(dataDir)
	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil, mapEnv(map[string]string{
		EnvStorageOption: StorageOptionDataDir,
	}))
	if err != nil {
		t.Fatalf("OpenAppOwnedState returned error: %v", err)
	}
	if state.SQLStore != nil {
		t.Fatalf("SQLStore = %#v, want nil for datadir", state.SQLStore)
	}
	if state.Provider == nil || state.Provider.Name() != StorageOptionDataDir {
		t.Fatalf("provider = %#v, want datadir", state.Provider)
	}
}

func TestStorageSyncDataDirToDatabaseCreatesBackupAndCopiesState(t *testing.T) {
	dataDir := t.TempDir()
	paths := testPaths(dataDir)
	source := registry.New(paths.RegistryFile, nil)
	workspace := models.WorkspaceConfig{ID: "workspace-1", Name: "Workspace", Path: t.TempDir(), BaselineBranch: "main", Sources: []string{"plans"}, CreatedAt: time.Now().UTC()}
	if err := writeYAMLFile(paths.RegistryFile, []models.WorkspaceConfig{workspace}); err != nil {
		t.Fatal(err)
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "item-1", WorkspaceID: workspace.ID, Branch: "main", Scope: "plans", Identifier: "PM-001", Title: "Plan", Status: models.StatusDraft, ItemPath: "plans/PM-001", SourceMode: "working_tree", Editable: true, UpdatedAt: time.Now().UTC()}}
	if err := itemindex.New(paths.PlanIndexFile).ReplaceWorkspaceBranch(workspace.ID, "main", []models.ItemDetail{item}, models.BranchScanMetadata{WorkspaceID: workspace.ID, Branch: "main", SourceMode: "working_tree", Editable: true, ScannedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := source.List(); err != nil {
		t.Fatal(err)
	}
	layout, _, err := canvas.NewFileRepository(paths.CanvasFile).ResolveDefault("", workspace.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := canvas.NewFileRepository(paths.CanvasFile).PatchPlacements(layout.ID, []canvas.PlacementPatch{{NodeID: "workspace", EntityRef: canvas.EntityRef{Kind: canvas.EntityWorkspace, WorkspaceID: workspace.ID}}}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := ai.NewFileSessionRecordRepository(paths.AISessionRecordsFile).Upsert(ai.SessionRecord{ID: "session-1", WorkspaceID: workspace.ID, Provider: "codex", Intent: "implement", RequestedBranch: "main", State: ai.StateInterrupted, StartedAt: now, LastKnownAt: now}); err != nil {
		t.Fatal(err)
	}
	service := NewStorageSyncService(Config{StorageOption: StorageOptionDataDir, Driver: StorageDriverFile, SQLitePath: paths.SQLiteDatabaseFile, Migrations: "auto"}, paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil)
	result, err := service.Sync(context.Background(), StorageSyncRequest{Direction: SyncDataDirToDatabase, Confirm: true})
	if err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}
	if !result.OK || result.BackupPath == "" || result.Summary["workspaces"] != 1 || result.Summary["items"] != 1 || result.Summary["canvasPlacements"] != 1 || result.Summary["sessionRecords"] != 1 {
		t.Fatalf("result = %#v", result)
	}
	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil, databaseEnv)
	if err != nil {
		t.Fatalf("OpenAppOwnedState returned error: %v", err)
	}
	defer state.SQLStore.Close()
	workspaces, err := state.Workspaces.List()
	if err != nil || len(workspaces) != 1 || workspaces[0].ID != workspace.ID {
		t.Fatalf("workspaces = %#v err=%v", workspaces, err)
	}
	items, err := state.Items.BranchItems(workspace.ID, "main")
	if err != nil || len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("items = %#v err=%v", items, err)
	}
	placements, err := state.Canvas.Placements(layout.ID)
	if err != nil || len(placements) != 1 || placements[0].NodeID != "workspace" {
		t.Fatalf("placements = %#v err=%v", placements, err)
	}
	records, err := state.SessionRecords.List(workspace.ID, "main")
	if err != nil || len(records) != 1 || records[0].ID != "session-1" {
		t.Fatalf("session records = %#v err=%v", records, err)
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("backup path missing: %v", err)
	}
}

func TestStorageSyncDatabaseToDataDirCreatesBackupAndCopiesState(t *testing.T) {
	dataDir := t.TempDir()
	paths := testPaths(dataDir)
	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil, databaseEnv)
	if err != nil {
		t.Fatalf("OpenAppOwnedState returned error: %v", err)
	}
	workspace := models.WorkspaceConfig{ID: "workspace-1", Name: "Workspace", Path: t.TempDir(), BaselineBranch: "main", Sources: []string{"plans"}, CreatedAt: time.Now().UTC()}
	if err := state.Workspaces.(*SQLiteWorkspaceRepository).upsert(workspace); err != nil {
		t.Fatal(err)
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "item-1", WorkspaceID: workspace.ID, Branch: "main", Scope: "plans", Identifier: "PM-001", Title: "Plan", Status: models.StatusDraft, ItemPath: "plans/PM-001", SourceMode: "working_tree", Editable: true, UpdatedAt: time.Now().UTC()}}
	if err := state.Items.ReplaceWorkspaceBranch(workspace.ID, "main", []models.ItemDetail{item}, models.BranchScanMetadata{WorkspaceID: workspace.ID, Branch: "main", SourceMode: "working_tree", Editable: true, ScannedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	layout, _, err := state.Canvas.ResolveDefault("", workspace.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.Canvas.PatchPlacements(layout.ID, []canvas.PlacementPatch{{NodeID: "workspace", EntityRef: canvas.EntityRef{Kind: canvas.EntityWorkspace, WorkspaceID: workspace.ID}}}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := state.SessionRecords.Upsert(ai.SessionRecord{ID: "session-1", WorkspaceID: workspace.ID, Provider: "codex", Intent: "implement", RequestedBranch: "main", State: ai.StateInterrupted, StartedAt: now, LastKnownAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := state.SQLStore.Close(); err != nil {
		t.Fatal(err)
	}

	if err := writeYAMLFile(paths.RegistryFile, []models.WorkspaceConfig{{ID: "old-workspace", Name: "Old", Path: t.TempDir(), BaselineBranch: "main", Sources: []string{"plans"}, CreatedAt: time.Now().UTC()}}); err != nil {
		t.Fatal(err)
	}
	service := NewStorageSyncService(Config{StorageOption: StorageOptionDatabase, Driver: StorageDriverSQLite, SQLitePath: paths.SQLiteDatabaseFile, Migrations: "auto"}, paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil)
	result, err := service.Sync(context.Background(), StorageSyncRequest{Direction: SyncDatabaseToDataDir, Confirm: true})
	if err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}
	if !result.OK || result.BackupPath == "" || result.Summary["workspaces"] != 1 || result.Summary["items"] != 1 || result.Summary["canvasPlacements"] != 1 || result.Summary["sessionRecords"] != 1 {
		t.Fatalf("result = %#v", result)
	}
	workspaces, err := registry.New(paths.RegistryFile, nil).List()
	if err != nil || len(workspaces) != 1 || workspaces[0].ID != workspace.ID {
		t.Fatalf("workspaces = %#v err=%v", workspaces, err)
	}
	items, err := itemindex.New(paths.PlanIndexFile).BranchItems(workspace.ID, "main")
	if err != nil || len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("items = %#v err=%v", items, err)
	}
	placements, err := canvas.NewFileRepository(paths.CanvasFile).Placements(layout.ID)
	if err != nil || len(placements) != 1 || placements[0].NodeID != "workspace" {
		t.Fatalf("placements = %#v err=%v", placements, err)
	}
	records, err := ai.NewFileSessionRecordRepository(paths.AISessionRecordsFile).List(workspace.ID, "main")
	if err != nil || len(records) != 1 || records[0].ID != "session-1" {
		t.Fatalf("session records = %#v err=%v", records, err)
	}
	if _, err := os.Stat(filepath.Join(result.BackupPath, filepath.Base(paths.RegistryFile))); err != nil {
		t.Fatalf("target backup missing registry file: %v", err)
	}
}

func TestSQLiteItemRepositoryDefaultsMissingUpdatedAt(t *testing.T) {
	dataDir := t.TempDir()
	paths := testPaths(dataDir)
	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil, databaseEnv)
	if err != nil {
		t.Fatalf("OpenAppOwnedState returned error: %v", err)
	}
	defer state.SQLStore.Close()

	scannedAt := time.Date(2026, 7, 17, 14, 6, 0, 0, time.UTC)
	item := models.ItemDetail{ItemSummary: models.ItemSummary{
		ID:          "item-1",
		WorkspaceID: "workspace-1",
		Branch:      "main",
		Scope:       "plans",
		Identifier:  "PM-001",
		Title:       "Plan",
		Status:      models.StatusDraft,
		ItemPath:    "plans/PM-001",
		SourceMode:  "snapshot",
	}}
	err = state.Items.ReplaceWorkspaceBranch("workspace-1", "main", []models.ItemDetail{item}, models.BranchScanMetadata{
		WorkspaceID: "workspace-1",
		Branch:      "main",
		SourceMode:  "snapshot",
		ScannedAt:   scannedAt,
	})
	if err != nil {
		t.Fatalf("ReplaceWorkspaceBranch returned error: %v", err)
	}

	stored, ok, err := state.Items.Get("item-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatal("stored item was not found")
	}
	if !stored.UpdatedAt.Equal(scannedAt) {
		t.Fatalf("updatedAt = %v, want %v", stored.UpdatedAt, scannedAt)
	}
	operational, err := state.Items.Query(itemindex.Query{WorkspaceID: "workspace-1"})
	if err != nil || len(operational) != 0 {
		t.Fatalf("snapshot leaked into operational query: items=%#v err=%v", operational, err)
	}
	review, err := state.Items.BranchItems("workspace-1", "main")
	if err != nil || len(review) != 1 || review[0].ID != item.ID {
		t.Fatalf("review items=%#v err=%v", review, err)
	}
}

func TestOpenAppOwnedStatePostgresIntegration(t *testing.T) {
	databaseURL := os.Getenv(EnvDatabaseURL)
	if databaseURL == "" {
		t.Skip("set KODE_STREAM_DATABASE_URL to run Postgres integration test")
	}
	admin, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("kode_stream_contract_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA "` + schema + `"`); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	defer admin.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`)
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	isolatedURL := parsed.String()
	dataDir := t.TempDir()
	paths := testPaths(dataDir)
	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeCloud}, gitadapter.New(), mapEnv(map[string]string{
		EnvStorageDriver: StorageDriverPostgres,
		EnvDatabaseURL:   isolatedURL,
	}))
	if err != nil {
		t.Fatalf("OpenAppOwnedState returned error: %v", err)
	}
	defer state.SQLStore.Close()
	health := state.SQLStore.Health(context.Background())
	if !health.OK || health.Driver != StorageDriverPostgres {
		t.Fatalf("health = %#v", health)
	}
	runSQLAppOwnedRepositoryContract(t, state, "postgres-shared")
	workspace := models.WorkspaceConfig{ID: "workspace-contract", Name: "Contract", Path: ".../contract", BaselineBranch: "main", RegistrationMode: models.WorkspaceRegistrationModeExisting, Sources: []string{"plans"}, ClonePathManaged: true, CreatedAt: time.Now().UTC()}
	if err := state.Workspaces.(*SQLiteWorkspaceRepository).upsert(workspace); err != nil {
		t.Fatal(err)
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "item-contract", WorkspaceID: workspace.ID, Branch: "main", Title: "Contract", SourceMode: "working_tree", Editable: true, UpdatedAt: time.Now().UTC()}}
	if err := state.Items.ReplaceWorkspaceBranch(workspace.ID, "main", []models.ItemDetail{item}, models.BranchScanMetadata{WorkspaceID: workspace.ID, Branch: "main", SourceMode: "working_tree", Editable: true, ScannedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	metadata, ok, err := state.Items.BranchScan(workspace.ID, "main")
	if err != nil || !ok || !metadata.Editable {
		t.Fatalf("Postgres Boolean round trip = %#v, %v, %v", metadata, ok, err)
	}
	if _, err := state.Audit.Append(models.AuditEvent{OwnerUserID: "owner-a", ActorUserID: "actor-a", WorkspaceID: workspace.ID, Operation: "contract", Status: models.AuditStatusSuccess}); err != nil {
		t.Fatal(err)
	}
	if events, err := state.Audit.QueryContext(context.Background(), audit.Query{OwnerUserID: "owner-a", WorkspaceID: workspace.ID, Limit: 1}); err != nil || len(events) != 1 {
		t.Fatalf("audit contract = %#v, %v", events, err)
	}
	ownedNavigation := state.Navigation.(navigation.OwnedRepository)
	if _, err := ownedNavigation.SaveFilterForOwner("owner-a", models.SavedFilter{Name: "Contract", Route: "/workstream"}); err != nil {
		t.Fatal(err)
	}
	if filters, err := ownedNavigation.FiltersForOwner("owner-b"); err != nil || len(filters) != 0 {
		t.Fatalf("navigation isolation = %#v, %v", filters, err)
	}
	runOwnedNavigationContract(t, state.Navigation.(navigation.SnapshotRepository))
	layout, _, err := state.Canvas.ResolveDefault("owner-a", workspace.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	placements, err := state.Canvas.PatchPlacements(layout.ID, []canvas.PlacementPatch{{NodeID: "workspace", EntityRef: canvas.EntityRef{Kind: canvas.EntityWorkspace, WorkspaceID: workspace.ID}, Position: canvas.Position{X: 1, Y: 2}}})
	if err != nil || len(placements) != 1 {
		t.Fatalf("Postgres Canvas contract=%#v err=%v", placements, err)
	}
	now := time.Now().UTC()
	record := ai.SessionRecord{ID: "session-contract", WorkspaceID: workspace.ID, Provider: "codex", Intent: "implement", RequestedBranch: "main", State: ai.StateInterrupted, StartedAt: now, LastKnownAt: now}
	if _, err := state.SessionRecords.Upsert(record); err != nil {
		t.Fatal(err)
	}
	if loaded, ok, err := state.SessionRecords.Get(record.ID); err != nil || !ok || loaded.State != record.State {
		t.Fatalf("Postgres session contract=%#v ok=%v err=%v", loaded, ok, err)
	}
	if _, err := state.Cloud.UpsertWorkspace(context.Background(), "owner-a", workspace); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := state.Cloud.GetWorkspace(context.Background(), "owner-a", workspace.ID); err != nil || !ok {
		t.Fatalf("Cloud workspace contract = %v, %v", ok, err)
	}
	runLegacyRepairRollbackContract(t, state, filepath.Join(t.TempDir(), "item-index.yaml"), "postgres-repair")

	legacySchema := fmt.Sprintf("kode_stream_boolean_upgrade_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA "` + legacySchema + `"`); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(`DROP SCHEMA IF EXISTS "` + legacySchema + `" CASCADE`)
	legacyParsed, _ := url.Parse(databaseURL)
	legacyQuery := legacyParsed.Query()
	legacyQuery.Set("search_path", legacySchema)
	legacyParsed.RawQuery = legacyQuery.Encode()
	legacyDB, err := sql.Open("pgx", legacyParsed.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ensureMigrations(context.Background(), legacyDB, StorageDriverPostgres, "auto", postgresMigrations()[:4]); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`ALTER TABLE workspaces ALTER COLUMN clone_path_managed DROP DEFAULT; ALTER TABLE workspaces ALTER COLUMN clone_path_managed TYPE INTEGER USING CASE WHEN clone_path_managed THEN 1 ELSE 0 END`,
		`ALTER TABLE branch_scans ALTER COLUMN editable TYPE INTEGER USING CASE WHEN editable THEN 1 ELSE 0 END`,
		`ALTER TABLE indexed_items ALTER COLUMN editable TYPE INTEGER USING CASE WHEN editable THEN 1 ELSE 0 END`,
		`ALTER TABLE canvas_placements ALTER COLUMN collapsed DROP DEFAULT; ALTER TABLE canvas_placements ALTER COLUMN collapsed TYPE INTEGER USING CASE WHEN collapsed THEN 1 ELSE 0 END`,
		`ALTER TABLE canvas_placements ALTER COLUMN hidden DROP DEFAULT; ALTER TABLE canvas_placements ALTER COLUMN hidden TYPE INTEGER USING CASE WHEN hidden THEN 1 ELSE 0 END`,
	} {
		if _, err := legacyDB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := legacyDB.Exec(`INSERT INTO indexed_items (id, workspace_id, branch, scope, identifier, title, status, item_path, source_mode, editable, metadata_json, updated_at) VALUES ('orphan', 'missing', 'main', 'plans', 'PM-1', 'Orphan', 'draft', 'plans/PM-1', 'working_tree', 1, '{}', CURRENT_TIMESTAMP);
INSERT INTO branch_scans (workspace_id, branch, source_mode, editable, scanned_at) VALUES ('missing', 'main', 'working_tree', 1, CURRENT_TIMESTAMP);
INSERT INTO scan_warnings (workspace_id, branch, item_path, code, message) VALUES ('missing', 'main', 'plans/PM-1', 'warning', 'preserve')`); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 51; index++ {
		if _, err := legacyDB.Exec(`INSERT INTO recent_items (item_id, workspace_id, title, route, opened_at) VALUES ($1, 'workspace', 'Recent', '/items/recent', $2)`, fmt.Sprintf("legacy-recent-%02d", index), time.Unix(int64(index), 0)); err != nil {
			t.Fatal(err)
		}
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatal(err)
	}
	legacyState, err := OpenAppOwnedState(testPaths(t.TempDir()), system.RuntimeConfig{Mode: models.RuntimeModeCloud}, gitadapter.New(), mapEnv(map[string]string{EnvStorageDriver: StorageDriverPostgres, EnvDatabaseURL: legacyParsed.String()}))
	if err != nil {
		t.Fatalf("integer Boolean schema upgrade: %v", err)
	}
	defer legacyState.SQLStore.Close()
	legacyWorkspace := models.WorkspaceConfig{ID: "legacy-boolean", Name: "Legacy", Path: ".../legacy", BaselineBranch: "main", Sources: []string{}, ClonePathManaged: true, CreatedAt: time.Now().UTC()}
	if err := legacyState.Workspaces.(*SQLiteWorkspaceRepository).upsert(legacyWorkspace); err != nil {
		t.Fatal(err)
	}
	if loaded, ok, err := legacyState.Workspaces.Get(legacyWorkspace.ID); err != nil || !ok || !loaded.ClonePathManaged {
		t.Fatalf("upgraded Boolean=%#v ok=%v err=%v", loaded, ok, err)
	}
	for _, table := range []string{"orphaned_indexed_items_v7", "orphaned_branch_scans_v7", "orphaned_scan_warnings_v7"} {
		var count int
		if err := legacyState.SQLStore.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("Postgres quarantine %s count=%d err=%v", table, count, err)
		}
	}
	var retained, pruned int
	if err := legacyState.SQLStore.db.QueryRow(`SELECT COUNT(*) FROM recent_items WHERE owner_user_id = 'local'`).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if err := legacyState.SQLStore.db.QueryRow(`SELECT COUNT(*) FROM pruned_recent_items_v6 WHERE owner_user_id = 'local'`).Scan(&pruned); err != nil {
		t.Fatal(err)
	}
	if retained != 50 || pruned != 1 {
		t.Fatalf("Postgres recent migration retained=%d pruned=%d", retained, pruned)
	}
}

func TestSQLiteSharedAppOwnedRepositoryContract(t *testing.T) {
	paths := testPaths(t.TempDir())
	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil, databaseEnv)
	if err != nil {
		t.Fatal(err)
	}
	defer state.SQLStore.Close()
	runSQLAppOwnedRepositoryContract(t, state, "sqlite-shared")
}

func runSQLAppOwnedRepositoryContract(t *testing.T, state *AppOwnedState, prefix string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Millisecond)
	workspace := models.WorkspaceConfig{ID: prefix + "-workspace", Name: "Contract", Path: ".../contract", BaselineBranch: "main", RegistrationMode: models.WorkspaceRegistrationModeExisting, Sources: []string{"plans"}, ClonePathManaged: true, CreatedAt: now}
	workspaces := state.Workspaces.(*SQLiteWorkspaceRepository)
	if err := workspaces.upsert(workspace); err != nil {
		t.Fatal(err)
	}
	loadedWorkspace, ok, err := workspaces.Get(workspace.ID)
	if err != nil || !ok || !loadedWorkspace.ClonePathManaged {
		t.Fatalf("workspace=%#v ok=%v err=%v", loadedWorkspace, ok, err)
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: prefix + "-item", WorkspaceID: workspace.ID, Branch: "main", Title: "Contract", SourceMode: "working_tree", Editable: true, UpdatedAt: now}}
	if err := state.Items.ReplaceWorkspaceBranch(workspace.ID, "main", []models.ItemDetail{item}, models.BranchScanMetadata{WorkspaceID: workspace.ID, Branch: "main", SourceMode: "working_tree", Editable: true, ScannedAt: now}); err != nil {
		t.Fatal(err)
	}
	if metadata, ok, err := state.Items.BranchScan(workspace.ID, "main"); err != nil || !ok || !metadata.Editable {
		t.Fatalf("branch=%#v ok=%v err=%v", metadata, ok, err)
	}
	if _, err := state.Audit.Append(models.AuditEvent{OwnerUserID: prefix + "-owner", ActorUserID: prefix + "-actor", WorkspaceID: workspace.ID, Operation: "contract", Status: models.AuditStatusSuccess, Time: now}); err != nil {
		t.Fatal(err)
	}
	if events, err := state.Audit.QueryContext(context.Background(), audit.Query{OwnerUserID: prefix + "-owner", WorkspaceID: workspace.ID, Limit: 1}); err != nil || len(events) != 1 {
		t.Fatalf("audit=%#v err=%v", events, err)
	}
	navigationRepository := state.Navigation.(navigation.SnapshotRepository)
	filter := models.SavedFilter{OwnerUserID: prefix + "-owner", ID: prefix + "-filter", Name: "Contract", Route: "/workstream", Filters: map[string]any{}, CreatedAt: now, UpdatedAt: now}
	recent := models.RecentItem{OwnerUserID: prefix + "-owner", ItemID: item.ID, WorkspaceID: workspace.ID, Title: item.Title, Route: "/items/" + item.ID, OpenedAt: now}
	if err := navigationRepository.RestoreFilter(filter); err != nil {
		t.Fatal(err)
	}
	if err := navigationRepository.RestoreRecent(recent); err != nil {
		t.Fatal(err)
	}
	if filters, err := navigationRepository.FiltersForOwner(filter.OwnerUserID); err != nil || len(filters) != 1 || !filters[0].CreatedAt.Equal(now) {
		t.Fatalf("filters=%#v err=%v", filters, err)
	}
	layout, _, err := state.Canvas.ResolveDefault(prefix+"-owner", workspace.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	if placements, err := state.Canvas.PatchPlacements(layout.ID, []canvas.PlacementPatch{{NodeID: "workspace", EntityRef: canvas.EntityRef{Kind: canvas.EntityWorkspace, WorkspaceID: workspace.ID}, Position: canvas.Position{X: 1, Y: 2}}}); err != nil || len(placements) != 1 {
		t.Fatalf("canvas=%#v err=%v", placements, err)
	}
	record := ai.SessionRecord{ID: prefix + "-session", WorkspaceID: workspace.ID, Provider: "codex", Intent: "implement", RequestedBranch: "main", State: ai.StateInterrupted, StartedAt: now, LastKnownAt: now}
	if _, err := state.SessionRecords.Upsert(record); err != nil {
		t.Fatal(err)
	}
	if loaded, ok, err := state.SessionRecords.Get(record.ID); err != nil || !ok || loaded.State != record.State {
		t.Fatalf("session=%#v ok=%v err=%v", loaded, ok, err)
	}
	cloud := &SQLCloudRepository{db: state.SQLStore.db, driver: state.SQLStore.driver, now: time.Now}
	workspace.OwnerUserID = prefix + "-owner"
	if _, err := cloud.UpsertWorkspace(context.Background(), workspace.OwnerUserID, workspace); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := cloud.GetWorkspace(context.Background(), workspace.OwnerUserID, workspace.ID); err != nil || !ok {
		t.Fatalf("cloud workspace ok=%v err=%v", ok, err)
	}
	if err := workspaces.DeleteWorkspaceState(workspace.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := workspaces.Get(workspace.ID); err != nil || ok {
		t.Fatalf("deleted workspace ok=%v err=%v", ok, err)
	}
	if items, err := state.Items.BranchItems(workspace.ID, "main"); err != nil || len(items) != 0 {
		t.Fatalf("deleted items=%#v err=%v", items, err)
	}
	if _, ok, err := state.SessionRecords.Get(record.ID); err != nil || ok {
		t.Fatalf("deleted session ok=%v err=%v", ok, err)
	}
}

func TestSQLCloudRepositoryPersistsAcrossReconstructionAndScopesOwners(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cloud.db")
	store, err := openSQLStore(Config{Driver: StorageDriverSQLite, SQLitePath: path, Migrations: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	repositoryA := &SQLCloudRepository{db: store.db, driver: store.driver, now: time.Now}
	workspace := models.WorkspaceConfig{ID: "workspace-1", Name: "Owned", OwnerUserID: "owner-a", Sources: []string{}}
	if _, err := repositoryA.UpsertWorkspace(context.Background(), "owner-a", workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := repositoryA.UpsertAgent(context.Background(), "owner-a", models.CloudAgent{ID: "agent-1", UserID: "owner-a", Status: "connected", LastSeenAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	repositoryB := &SQLCloudRepository{db: store.db, driver: store.driver, now: time.Now}
	loaded, ok, err := repositoryB.GetWorkspace(context.Background(), "owner-a", workspace.ID)
	if err != nil || !ok || loaded.Name != workspace.Name {
		t.Fatalf("reconstructed read = %#v, %v, %v", loaded, ok, err)
	}
	if _, ok, err := repositoryB.GetWorkspace(context.Background(), "owner-b", workspace.ID); err != nil || ok {
		t.Fatalf("cross-owner read = %v, %v", ok, err)
	}
	if agents, err := repositoryB.ListAgents(context.Background(), "owner-a"); err != nil || len(agents) != 1 {
		t.Fatalf("agents = %#v, %v", agents, err)
	}
	if err := repositoryA.SaveEncryptedConnection(context.Background(), "owner-a", "github", "v1:ciphertext-only"); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := store.db.QueryRow(`SELECT encrypted_credentials FROM cloud_provider_connections WHERE owner_user_id = 'owner-a' AND provider_instance_id = 'github'`).Scan(&stored); err != nil || stored != "v1:ciphertext-only" {
		t.Fatalf("stored credential = %q, %v", stored, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteAuditQueryFiltersBeforeLimitWithAdversarialInterleaving(t *testing.T) {
	store, err := openSQLStore(Config{Driver: StorageDriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "audit.db"), Migrations: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	repository := &SQLiteAuditRepository{db: store.db, driver: store.driver, now: time.Now}
	for index := 0; index < 30; index++ {
		workspace := "other"
		if index == 0 || index == 5 || index == 10 {
			workspace = "wanted"
		}
		if _, err := repository.Append(models.AuditEvent{OwnerUserID: "owner", WorkspaceID: workspace, Operation: "event", Time: time.Unix(int64(index), 0)}); err != nil {
			t.Fatal(err)
		}
	}
	events, err := repository.QueryContext(context.Background(), audit.Query{OwnerUserID: "owner", WorkspaceID: "wanted", Limit: 3})
	if err != nil || len(events) != 3 {
		t.Fatalf("events=%#v err=%v", events, err)
	}
}

func TestNavigationRepositoryContractFileAndSQLite(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		dir := t.TempDir()
		runOwnedNavigationContract(t, navigation.New(filepath.Join(dir, "filters.yaml"), filepath.Join(dir, "recents.yaml")))
	})
	t.Run("sqlite", func(t *testing.T) {
		store, err := openSQLStore(Config{Driver: StorageDriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "navigation.db"), Migrations: "auto"})
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		runOwnedNavigationContract(t, &SQLiteNavigationRepository{db: store.db, driver: store.driver, now: time.Now})
	})
}

func runOwnedNavigationContract(t *testing.T, repository navigation.SnapshotRepository) {
	t.Helper()
	created, err := repository.SaveFilterForOwner("owner-a", models.SavedFilter{Name: "First", Route: "/workstream"})
	if err != nil || created.ID == "" {
		t.Fatalf("create filter=%#v err=%v", created, err)
	}
	created.Name = "Updated"
	updated, err := repository.SaveFilterForOwner("owner-a", created)
	if err != nil || updated.Name != "Updated" || updated.CreatedAt != created.CreatedAt {
		t.Fatalf("update filter=%#v err=%v", updated, err)
	}
	if filters, err := repository.FiltersForOwner("owner-b"); err != nil || len(filters) != 0 {
		t.Fatalf("owner isolation=%#v err=%v", filters, err)
	}
	if deleted, err := repository.DeleteFilterForOwner("owner-b", created.ID); err != nil || deleted {
		t.Fatalf("cross-owner delete=%v err=%v", deleted, err)
	}

	for index := 0; index < 51; index++ {
		if err := repository.RecordRecentForOwner("owner-a", models.RecentItem{ItemID: fmt.Sprintf("item-%02d", index), WorkspaceID: "workspace", Title: "Item", Route: "/items/x"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.RecordRecentForOwner("owner-b", models.RecentItem{ItemID: "item-50", WorkspaceID: "workspace", Title: "Other", Route: "/items/x"}); err != nil {
		t.Fatal(err)
	}
	recents, err := repository.RecentsForOwner("owner-a", 100)
	if err != nil || len(recents) != 50 {
		t.Fatalf("retention=%d err=%v", len(recents), err)
	}
	other, err := repository.RecentsForOwner("owner-b", 100)
	if err != nil || len(other) != 1 {
		t.Fatalf("owner retention=%#v err=%v", other, err)
	}

	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	openedAt := updatedAt.Add(time.Hour)
	if err := repository.RestoreFilter(models.SavedFilter{OwnerUserID: "owner-a", ID: "snapshot-filter", Name: "Snapshot", Route: "/workstream", Filters: map[string]any{}, CreatedAt: createdAt, UpdatedAt: updatedAt}); err != nil {
		t.Fatal(err)
	}
	if err := repository.RestoreRecent(models.RecentItem{OwnerUserID: "owner-a", ItemID: "snapshot-item", WorkspaceID: "workspace", Title: "Snapshot", Route: "/items/snapshot-item", OpenedAt: openedAt}); err != nil {
		t.Fatal(err)
	}
	filters, _ := repository.FiltersForOwner("owner-a")
	recents, _ = repository.RecentsForOwner("owner-a", 100)
	foundFilter, foundRecent := false, false
	for _, filter := range filters {
		if filter.ID == "snapshot-filter" && filter.CreatedAt.Equal(createdAt) && filter.UpdatedAt.Equal(updatedAt) {
			foundFilter = true
		}
	}
	for _, recent := range recents {
		if recent.ItemID == "snapshot-item" && recent.OpenedAt.Equal(openedAt) {
			foundRecent = true
		}
	}
	if !foundFilter || !foundRecent {
		t.Fatalf("snapshot restore filter=%v recent=%v", foundFilter, foundRecent)
	}
}

func TestSQLiteNavigationRepositoryOwnerContractAndRetention(t *testing.T) {
	store, err := openSQLStore(Config{Driver: StorageDriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "navigation.db"), Migrations: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	repository := &SQLiteNavigationRepository{db: store.db, driver: store.driver, now: time.Now}
	filter, err := repository.SaveFilterForOwner("owner-a", models.SavedFilter{Name: "A", Route: "/workstream"})
	if err != nil {
		t.Fatal(err)
	}
	if deleted, err := repository.DeleteFilterForOwner("owner-b", filter.ID); err != nil || deleted {
		t.Fatalf("cross-owner delete = %v, %v", deleted, err)
	}
	for index := 0; index < 51; index++ {
		if err := repository.RecordRecentForOwner("owner-a", models.RecentItem{ItemID: fmt.Sprintf("item-%02d", index), Title: "item"}); err != nil {
			t.Fatal(err)
		}
	}
	recents, err := repository.RecentsForOwner("owner-a", 0)
	if err != nil || len(recents) != 50 {
		t.Fatalf("recents = %d, %v", len(recents), err)
	}
}

func TestSQLWorkspaceDeletionRollsBackAllOwnedStateOnFailure(t *testing.T) {
	store, err := openSQLStore(Config{Driver: StorageDriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "delete.db"), Migrations: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	workspaces := newSQLiteWorkspaceRepository(store, testPaths(t.TempDir()), nil)
	workspace := models.WorkspaceConfig{ID: "workspace-delete", Name: "Delete", Path: ".../delete", BaselineBranch: "main", Sources: []string{}, CreatedAt: time.Now().UTC()}
	if err := workspaces.upsert(workspace); err != nil {
		t.Fatal(err)
	}
	items := &SQLiteItemRepository{db: store.db, driver: store.driver}
	if err := items.ReplaceWorkspaceBranch(workspace.ID, "main", []models.ItemDetail{{ItemSummary: models.ItemSummary{ID: "item", WorkspaceID: workspace.ID, Branch: "main", Title: "Item", UpdatedAt: time.Now().UTC()}}}, models.BranchScanMetadata{WorkspaceID: workspace.ID, Branch: "main", ScannedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`CREATE TRIGGER fail_workspace_delete BEFORE DELETE ON workspaces BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := workspaces.DeleteWorkspaceState(workspace.ID); err == nil {
		t.Fatal("expected injected deletion failure")
	}
	if _, ok, err := workspaces.Get(workspace.ID); err != nil || !ok {
		t.Fatalf("workspace was not rolled back: %v, %v", ok, err)
	}
	if branchItems, err := items.BranchItems(workspace.ID, "main"); err != nil || len(branchItems) != 1 {
		t.Fatalf("items were not rolled back: %#v, %v", branchItems, err)
	}
}

func TestLegacyItemImportPreservesSameIDAcrossBranchesWarningsAndTimestamps(t *testing.T) {
	paths := testPaths(t.TempDir())
	legacy := itemindex.New(paths.PlanIndexFile)
	updated := time.Date(2026, 8, 1, 2, 3, 4, 123000000, time.UTC)
	for _, branch := range []string{"main", "feature"} {
		item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "shared-id", WorkspaceID: "workspace", Branch: branch, Title: branch, SourceMode: "snapshot", Editable: false, UpdatedAt: updated}}
		metadata := models.BranchScanMetadata{WorkspaceID: "workspace", Branch: branch, SourceMode: "snapshot", Editable: false, ScannedAt: updated, Warnings: []models.ScanWarning{{ItemPath: branch + "/plan.md", Message: "warning " + branch}}}
		if err := legacy.ReplaceWorkspaceBranch("workspace", branch, []models.ItemDetail{item}, metadata); err != nil {
			t.Fatal(err)
		}
	}
	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil, mapEnv(map[string]string{EnvStorageOption: StorageOptionDatabase, EnvStorageDriver: StorageDriverSQLite, EnvSQLitePath: paths.SQLiteDatabaseFile}))
	if err != nil {
		t.Fatal(err)
	}
	defer state.SQLStore.Close()
	for _, branch := range []string{"main", "feature"} {
		items, err := state.Items.BranchItems("workspace", branch)
		if err != nil || len(items) != 1 || items[0].Title != branch || !items[0].UpdatedAt.Equal(updated) {
			t.Fatalf("branch %s items=%#v err=%v", branch, items, err)
		}
		metadata, ok, err := state.Items.BranchScan("workspace", branch)
		if err != nil || !ok || len(metadata.Warnings) != 1 || !metadata.ScannedAt.Equal(updated) {
			t.Fatalf("branch %s metadata=%#v ok=%v err=%v", branch, metadata, ok, err)
		}
	}
	if err := ImportLegacyFiles(paths, nil, state); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyItemRepairRollsBackPartialReplacementAndRetrySucceeds(t *testing.T) {
	paths := testPaths(t.TempDir())
	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil, mapEnv(map[string]string{EnvStorageOption: StorageOptionDatabase, EnvStorageDriver: StorageDriverSQLite, EnvSQLitePath: paths.SQLiteDatabaseFile}))
	if err != nil {
		t.Fatal(err)
	}
	defer state.SQLStore.Close()
	runLegacyRepairRollbackContract(t, state, paths.PlanIndexFile, "workspace")
}

func TestConcurrentLegacyItemImportPublishesOneIdempotentSnapshot(t *testing.T) {
	paths := testPaths(t.TempDir())
	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil, mapEnv(map[string]string{EnvStorageOption: StorageOptionDatabase, EnvStorageDriver: StorageDriverSQLite, EnvSQLitePath: paths.SQLiteDatabaseFile}))
	if err != nil {
		t.Fatal(err)
	}
	defer state.SQLStore.Close()
	snapshot := fileIndexState{Items: []models.ItemDetail{{ItemSummary: models.ItemSummary{ID: "item", WorkspaceID: "workspace", Branch: "main", Title: "Concurrent", UpdatedAt: time.Now().UTC()}}}}
	if err := writeYAMLFile(paths.PlanIndexFile, snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := state.SQLStore.db.Exec(`DELETE FROM import_status WHERE source_name = 'item-index.yaml'`); err != nil {
		t.Fatal(err)
	}
	errorsChannel := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() { defer wait.Done(); errorsChannel <- importLegacyItemsAtomic(paths.PlanIndexFile, state, false) }()
	}
	wait.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("concurrent import error: %v", err)
		}
	}
	items, err := state.Items.BranchItems("workspace", "main")
	if err != nil || len(items) != 1 || items[0].ID != "item" {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	var completed int
	if err := state.SQLStore.db.QueryRow(`SELECT COUNT(*) FROM import_status WHERE source_name = 'item-index.yaml'`).Scan(&completed); err != nil || completed != 1 {
		t.Fatalf("completion rows=%d err=%v", completed, err)
	}
}

func runLegacyRepairRollbackContract(t *testing.T, state *AppOwnedState, path, workspaceID string) {
	t.Helper()
	existing := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "existing", WorkspaceID: workspaceID, Branch: "main", Title: "Existing", UpdatedAt: time.Now().UTC()}}
	if err := state.Items.ReplaceWorkspaceBranch(workspaceID, "main", []models.ItemDetail{existing}, models.BranchScanMetadata{WorkspaceID: workspaceID, Branch: "main", ScannedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	duplicate := fileIndexState{Items: []models.ItemDetail{
		{ItemSummary: models.ItemSummary{ID: "duplicate", WorkspaceID: workspaceID, Branch: "main", Title: "One", UpdatedAt: time.Now().UTC()}},
		{ItemSummary: models.ItemSummary{ID: "duplicate", WorkspaceID: workspaceID, Branch: "main", Title: "Two", UpdatedAt: time.Now().UTC()}},
	}}
	if err := writeYAMLFile(path, duplicate); err != nil {
		t.Fatal(err)
	}
	if err := RepairLegacyItemImport(path, state); err == nil {
		t.Fatal("expected duplicate import failure")
	}
	items, err := state.Items.BranchItems(workspaceID, "main")
	if err != nil || len(items) != 1 || items[0].ID != existing.ID {
		t.Fatalf("rollback items=%#v err=%v", items, err)
	}
	valid := fileIndexState{Items: duplicate.Items[:1]}
	if err := writeYAMLFile(path, valid); err != nil {
		t.Fatal(err)
	}
	if err := RepairLegacyItemImport(path, state); err != nil {
		t.Fatal(err)
	}
	items, err = state.Items.BranchItems(workspaceID, "main")
	if err != nil || len(items) != 1 || items[0].ID != "duplicate" {
		t.Fatalf("retry items=%#v err=%v", items, err)
	}
}

func TestStorageSyncRejectsActiveTargetAndConcurrentOperation(t *testing.T) {
	paths := testPaths(t.TempDir())
	service := NewStorageSyncService(Config{StorageOption: StorageOptionDatabase, Driver: StorageDriverSQLite, SQLitePath: paths.SQLiteDatabaseFile, Migrations: "auto"}, paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil)
	_, err := service.Sync(context.Background(), StorageSyncRequest{Direction: SyncDataDirToDatabase, Confirm: true})
	var syncErr *SyncError
	if !errors.As(err, &syncErr) || syncErr.Kind != SyncErrorConflict {
		t.Fatalf("active target error = %#v", err)
	}
	service.running.Store(true)
	_, err = service.Sync(context.Background(), StorageSyncRequest{Direction: SyncDatabaseToDataDir, Confirm: true})
	if !errors.As(err, &syncErr) || syncErr.Kind != SyncErrorConflict {
		t.Fatalf("concurrent error = %#v", err)
	}
}

func TestStorageSyncUsesConfiguredSQLiteTargetPath(t *testing.T) {
	paths := testPaths(t.TempDir())
	custom := filepath.Join(paths.Dir, "custom", "state.db")
	if err := writeYAMLFile(paths.RegistryFile, []models.WorkspaceConfig{}); err != nil {
		t.Fatal(err)
	}
	service := NewStorageSyncService(Config{StorageOption: StorageOptionDataDir, Driver: StorageDriverFile, SQLitePath: custom, Migrations: "auto"}, paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil)
	if _, err := service.Sync(context.Background(), StorageSyncRequest{Direction: SyncDataDirToDatabase, Confirm: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(custom); err != nil {
		t.Fatalf("custom SQLite target missing: %v", err)
	}
	if _, err := os.Stat(paths.SQLiteDatabaseFile); !os.IsNotExist(err) {
		t.Fatalf("default SQLite path was unexpectedly used: %v", err)
	}
}

func TestDatabaseBackupUsesConsistentSQLiteSnapshotInWALMode(t *testing.T) {
	paths := testPaths(t.TempDir())
	db, err := sql.Open("sqlite", paths.SQLiteDatabaseFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; CREATE TABLE records (id INTEGER PRIMARY KEY, value TEXT); INSERT INTO records(value) VALUES ('committed-in-wal')`); err != nil {
		t.Fatal(err)
	}
	backup, err := backupDatabaseTarget(paths, paths.SQLiteDatabaseFile, SyncDataDirToDatabase, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	backupDB, err := sql.Open("sqlite", filepath.Join(backup, filepath.Base(paths.SQLiteDatabaseFile)))
	if err != nil {
		t.Fatal(err)
	}
	defer backupDB.Close()
	var value string
	if err := backupDB.QueryRow(`SELECT value FROM records`).Scan(&value); err != nil || value != "committed-in-wal" {
		t.Fatalf("backup value=%q err=%v", value, err)
	}
	if _, err := os.Stat(filepath.Join(backup, filepath.Base(paths.SQLiteDatabaseFile)+"-wal")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("standalone snapshot unexpectedly depends on WAL: %v", err)
	}
}

func TestStorageSyncFailureBeforePublishLeavesTargetAndBackupRecoverable(t *testing.T) {
	paths := testPaths(t.TempDir())
	custom := filepath.Join(paths.Dir, "custom.db")
	if err := os.WriteFile(custom, []byte("original-target"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeYAMLFile(paths.RegistryFile, []models.WorkspaceConfig{}); err != nil {
		t.Fatal(err)
	}
	service := NewStorageSyncService(Config{StorageOption: StorageOptionDataDir, Driver: StorageDriverFile, SQLitePath: custom, Migrations: "auto"}, paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil)
	service.beforePhase = func(phase string) error {
		if phase == "before_publish" {
			return errors.New("injected publish failure")
		}
		return nil
	}
	if _, err := service.Sync(context.Background(), StorageSyncRequest{Direction: SyncDataDirToDatabase, Confirm: true}); err == nil {
		t.Fatal("expected injected failure")
	}
	data, err := os.ReadFile(custom)
	if err != nil || string(data) != "original-target" {
		t.Fatalf("target changed after staged failure: %q, %v", data, err)
	}
	backups, err := filepath.Glob(filepath.Join(paths.Dir, "backups", "storage-sync", "*", filepath.Base(custom)))
	if err != nil || len(backups) != 1 {
		t.Fatalf("recoverable backup = %#v, %v", backups, err)
	}
}

func TestPublishDataDirStageRollsBackEveryMidPublicationFailure(t *testing.T) {
	fileNames := []string{"workspaces.yaml", "item-index.yaml", "audit-log.jsonl", "saved-filters.yaml", "recent-items.yaml", "ai-settings.yaml", "canvases.yaml", "ai-session-records.yaml"}
	for _, failName := range fileNames {
		t.Run(failName, func(t *testing.T) {
			root := t.TempDir()
			target := testPaths(filepath.Join(root, "target"))
			stage := syncStagePaths(target, filepath.Join(root, "stage"))
			backup := filepath.Join(root, "backup")
			for _, directory := range []string{target.Dir, stage.Dir, backup} {
				if err := os.MkdirAll(directory, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range fileNames {
				if err := os.WriteFile(filepath.Join(target.Dir, name), []byte("old:"+name), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(stage.Dir, name), []byte("new:"+name), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(backup, name), []byte("old:"+name), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := publishDataDirStage(stage, target, backup, func(phase string) error {
				if phase == "publish:"+failName {
					return errors.New("injected mid-publication failure")
				}
				return nil
			})
			if err == nil {
				t.Fatal("expected publication failure")
			}
			for _, name := range fileNames {
				data, readErr := os.ReadFile(filepath.Join(target.Dir, name))
				if readErr != nil || string(data) != "old:"+name {
					t.Fatalf("%s after rollback = %q, %v", name, data, readErr)
				}
				backupData, backupErr := os.ReadFile(filepath.Join(backup, name))
				if backupErr != nil || string(backupData) != "old:"+name {
					t.Fatalf("backup %s = %q, %v", name, backupData, backupErr)
				}
			}
		})
	}
}

func TestPublishDataDirStageReportsRestorationFailure(t *testing.T) {
	root := t.TempDir()
	target := testPaths(filepath.Join(root, "target"))
	stage := syncStagePaths(target, filepath.Join(root, "stage"))
	backup := filepath.Join(root, "backup")
	for _, directory := range []string{target.Dir, stage.Dir, backup} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"workspaces.yaml", "item-index.yaml", "audit-log.jsonl", "saved-filters.yaml", "recent-items.yaml", "ai-settings.yaml", "canvases.yaml", "ai-session-records.yaml"} {
		_ = os.WriteFile(filepath.Join(target.Dir, name), []byte("old"), 0o600)
		_ = os.WriteFile(filepath.Join(stage.Dir, name), []byte("new"), 0o600)
		_ = os.WriteFile(filepath.Join(backup, name), []byte("old"), 0o600)
	}
	err := publishDataDirStage(stage, target, backup, func(phase string) error {
		switch phase {
		case "publish:item-index.yaml":
			return errors.New("publish failed")
		case "restore:workspaces.yaml":
			return errors.New("restore failed")
		default:
			return nil
		}
	})
	if err == nil || !strings.Contains(err.Error(), "publish failed") || !strings.Contains(err.Error(), "restore failed") {
		t.Fatalf("combined rollback error = %v", err)
	}
}

func TestPublishDataDirStageCancellationRollsBackWithoutCancellingRestoration(t *testing.T) {
	root := t.TempDir()
	target := testPaths(filepath.Join(root, "target"))
	stage := syncStagePaths(target, filepath.Join(root, "stage"))
	backup := filepath.Join(root, "backup")
	fileNames := []string{"workspaces.yaml", "item-index.yaml", "audit-log.jsonl", "saved-filters.yaml", "recent-items.yaml", "ai-settings.yaml", "canvases.yaml", "ai-session-records.yaml"}
	for _, directory := range []string{target.Dir, stage.Dir, backup} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range fileNames {
		if err := os.WriteFile(filepath.Join(target.Dir, name), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stage.Dir, name), []byte("new"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backup, name), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cancelled := false
	err := publishDataDirStage(stage, target, backup, func(phase string) error {
		if strings.HasPrefix(phase, "restore:") {
			return nil
		}
		if cancelled {
			return context.Canceled
		}
		cancelled = true
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	for _, name := range fileNames {
		data, readErr := os.ReadFile(filepath.Join(target.Dir, name))
		if readErr != nil || string(data) != "old" {
			t.Fatalf("%s after cancellation=%q err=%v", name, data, readErr)
		}
	}
}

func testPaths(dataDir string) system.Paths {
	return system.Paths{
		Dir:                  dataDir,
		RegistryFile:         filepath.Join(dataDir, "workspaces.yaml"),
		PlanIndexFile:        filepath.Join(dataDir, "item-index.yaml"),
		SQLiteDatabaseFile:   filepath.Join(dataDir, "kode-stream.db"),
		KnowledgeIndexFile:   filepath.Join(dataDir, "knowledge-index.yaml"),
		AuditLogFile:         filepath.Join(dataDir, "audit-log.jsonl"),
		SavedFiltersFile:     filepath.Join(dataDir, "saved-filters.yaml"),
		RecentItemsFile:      filepath.Join(dataDir, "recent-items.yaml"),
		AISettingsFile:       filepath.Join(dataDir, "ai-settings.yaml"),
		CanvasFile:           filepath.Join(dataDir, "canvases.yaml"),
		AISessionRecordsFile: filepath.Join(dataDir, "ai-session-records.yaml"),
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func emptyEnv(string) string { return "" }

func databaseEnv(key string) string {
	if key == EnvStorageOption {
		return StorageOptionDatabase
	}
	return ""
}

func mapEnv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}
