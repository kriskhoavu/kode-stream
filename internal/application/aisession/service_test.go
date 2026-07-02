package aisession

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"plan-manager/internal/aisettings"
)

func TestSettingsRecommendFirstDetectedProviderAndTerminal(t *testing.T) {
	service := newTestService(t)
	service.lookPath = func(name string) (string, error) {
		if name == "claude" || name == "wezterm" {
			return "/bin/" + name, nil
		}
		return "", errors.New("missing")
	}
	service.stat = func(path string) (os.FileInfo, error) { return nil, os.ErrNotExist }

	settings, err := service.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.DefaultProvider != "claude" || settings.DefaultTerminal != "wezterm" {
		t.Fatalf("defaults = %q, %q", settings.DefaultProvider, settings.DefaultTerminal)
	}
}

func TestCapabilitiesReportDetectedDisabledAndMissingTools(t *testing.T) {
	service := newTestService(t)
	service.lookPath = func(name string) (string, error) {
		if name == "codex" {
			return "/usr/local/bin/codex", nil
		}
		return "", errors.New("missing")
	}
	service.stat = func(path string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	settings, err := service.Settings()
	if err != nil {
		t.Fatal(err)
	}
	template := settings.Providers["claude"]
	template.Enabled = false
	settings.Providers["claude"] = template
	if _, err := service.Save(settings); err != nil {
		t.Fatal(err)
	}

	capabilities, err := service.Capabilities()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Capability{}
	for _, capability := range capabilities {
		byID[capability.ID] = capability
	}
	if !byID["codex"].Detected || !byID["codex"].Configured {
		t.Fatalf("codex = %#v", byID["codex"])
	}
	if byID["claude"].Configured || byID["claude"].Reason != "disabled in settings" {
		t.Fatalf("claude = %#v", byID["claude"])
	}
	if byID["copilot"].Detected || byID["copilot"].Reason == "" {
		t.Fatalf("copilot = %#v", byID["copilot"])
	}
}

func TestSaveRejectsDisabledDefault(t *testing.T) {
	service := newTestService(t)
	settings, err := service.Settings()
	if err != nil {
		t.Fatal(err)
	}
	template := settings.Providers[settings.DefaultProvider]
	template.Enabled = false
	settings.Providers[settings.DefaultProvider] = template
	if _, err := service.Save(settings); err == nil {
		t.Fatal("expected disabled default to fail")
	}
}

func TestSettingsMigratesLegacyBehaviorPrompt(t *testing.T) {
	service := newTestService(t)
	legacy := aisettings.Settings{
		DefaultProvider: "codex", DefaultTerminal: "wezterm",
		Providers: map[string]aisettings.LaunchTemplate{"codex": {Enabled: true, Executable: "codex", Args: []string{"Read {contextFile} and follow its {intent} instructions for {identifier}."}}},
		Terminals: map[string]aisettings.LaunchTemplate{"wezterm": {Enabled: true, Executable: "wezterm"}},
	}
	if _, err := service.store.Save(legacy); err != nil {
		t.Fatal(err)
	}
	settings, err := service.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if got := settings.Providers["codex"].Args; len(got) != 1 || strings.Contains(got[0], "intent") || strings.Contains(got[0], "contextFile") || !strings.Contains(got[0], "{itemPath}") {
		t.Fatalf("migrated args = %#v", got)
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	service := New(aisettings.New(filepath.Join(t.TempDir(), "ai-settings.yaml")))
	service.goos = "darwin"
	return service
}
