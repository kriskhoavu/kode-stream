package ai

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStoreSavesAndLoadsPrivateSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "ai-settings.yaml")
	store := NewSettingsRepository(path)
	input := Settings{
		DefaultProvider: "codex",
		DefaultTerminal: "wezterm",
		Providers:       map[string]LaunchTemplate{"codex": {Enabled: true, Executable: "codex", Args: []string{"Read {contextFile}"}}},
		Terminals:       map[string]LaunchTemplate{"wezterm": {Enabled: true, Executable: "wezterm"}},
	}

	saved, err := store.Save(input)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultProvider != saved.DefaultProvider || loaded.Providers["codex"].Executable != "codex" {
		t.Fatalf("loaded settings = %#v", loaded)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("settings mode = %o", info.Mode().Perm())
		}
	}
}

func TestStoreMissingFileReturnsEmptySettings(t *testing.T) {
	settings, err := NewSettingsRepository(filepath.Join(t.TempDir(), "missing.yaml")).Load()
	if err != nil || settings.Providers == nil || settings.Terminals == nil {
		t.Fatalf("settings=%#v err=%v", settings, err)
	}
}

func TestValidateRejectsUnsafeOrUnknownTemplates(t *testing.T) {
	tests := []Settings{
		{Providers: map[string]LaunchTemplate{"codex": {Enabled: true}}},
		{Providers: map[string]LaunchTemplate{"codex": {Enabled: true, Executable: "codex", Args: []string{"{unknown}"}}}},
		{Providers: map[string]LaunchTemplate{"codex": {Enabled: true, Executable: "codex\nrm"}}},
		{DefaultProvider: "missing", Providers: map[string]LaunchTemplate{}},
	}
	for _, settings := range tests {
		if err := Validate(normalize(settings)); err == nil {
			t.Fatalf("expected settings to fail: %#v", settings)
		}
	}
}

func TestLoadReportsMalformedYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai-settings.yaml")
	if err := os.WriteFile(path, []byte("providers: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewSettingsRepository(path).Load()
	if err == nil || !strings.Contains(err.Error(), "read AI settings") {
		t.Fatalf("err = %v", err)
	}
}
