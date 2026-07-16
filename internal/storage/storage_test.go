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

func TestResolveConfigDefaultsLocalModeToSQLite(t *testing.T) {
	paths := system.Paths{SQLiteDatabaseFile: filepath.Join(t.TempDir(), "kode-stream.db")}
	config, err := ResolveConfig(system.RuntimeConfig{Mode: models.RuntimeModeLocal}, paths, emptyEnv)
	if err != nil {
		t.Fatalf("ResolveConfig returned error: %v", err)
	}
	if config.Driver != StorageDriverSQLite {
		t.Fatalf("driver = %q, want %q", config.Driver, StorageDriverSQLite)
	}
	if config.SQLitePath != paths.SQLiteDatabaseFile {
		t.Fatalf("sqlite path = %q, want %q", config.SQLitePath, paths.SQLiteDatabaseFile)
	}
	if config.Migrations != "auto" {
		t.Fatalf("migrations = %q, want auto", config.Migrations)
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
	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil, emptyEnv)
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
		EnvMigrations: "manual",
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

	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, git, emptyEnv)
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

func mapEnv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}
