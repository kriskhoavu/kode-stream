package storage

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"kode-stream/internal/ai"
	"kode-stream/internal/audit"
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
	if health.Driver != StorageDriverSQLite || health.MigrationVersion != 1 {
		t.Fatalf("health = %#v, want sqlite version 1", health)
	}
	for _, table := range []string{"workspaces", "branch_scans", "indexed_items", "import_status"} {
		var name string
		err := state.SQLStore.db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s was not created: %v", table, err)
		}
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
	service := NewStorageSyncService(Config{StorageOption: StorageOptionDataDir, Driver: StorageDriverFile, SQLitePath: paths.SQLiteDatabaseFile, Migrations: "auto"}, paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil)
	result, err := service.Sync(context.Background(), StorageSyncRequest{Direction: SyncDataDirToDatabase, Confirm: true})
	if err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}
	if !result.OK || result.BackupPath == "" || result.Summary["workspaces"] != 1 || result.Summary["items"] != 1 {
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
	if !result.OK || result.BackupPath == "" || result.Summary["workspaces"] != 1 || result.Summary["items"] != 1 {
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
}

func TestOpenAppOwnedStatePostgresIntegration(t *testing.T) {
	databaseURL := os.Getenv(EnvDatabaseURL)
	if databaseURL == "" {
		t.Skip("set KODE_STREAM_DATABASE_URL to run Postgres integration test")
	}
	dataDir := t.TempDir()
	paths := testPaths(dataDir)
	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeCloud}, gitadapter.New(), mapEnv(map[string]string{
		EnvStorageDriver: StorageDriverPostgres,
		EnvDatabaseURL:   databaseURL,
	}))
	if err != nil {
		t.Fatalf("OpenAppOwnedState returned error: %v", err)
	}
	defer state.SQLStore.Close()
	health := state.SQLStore.Health(context.Background())
	if !health.OK || health.Driver != StorageDriverPostgres {
		t.Fatalf("health = %#v", health)
	}
}

func testPaths(dataDir string) system.Paths {
	return system.Paths{
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
