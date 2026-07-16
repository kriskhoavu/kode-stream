package storage

import (
	"context"
	"path/filepath"
	"testing"

	"kode-stream/internal/common/models"
	"kode-stream/internal/system"
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

func emptyEnv(string) string { return "" }

func mapEnv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}
