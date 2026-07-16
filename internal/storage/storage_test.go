package storage

import (
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

func emptyEnv(string) string { return "" }

func mapEnv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}
