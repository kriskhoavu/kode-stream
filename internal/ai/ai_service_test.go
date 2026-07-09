package ai

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestProviderCapabilitiesReturnsProviderScopedFallbackCatalog(t *testing.T) {
	service := newTestService(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".codex", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "skills", "implementation-plan.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "agents", "reviewer.md"), []byte("# agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := service.ProviderCapabilities("codex", "")
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Provider != "codex" || len(catalog.Skills) == 0 || len(catalog.Agents) == 0 {
		t.Fatalf("catalog = %#v", catalog)
	}
	if catalog.SupportsNativeSelection || !catalog.SupportsPromptFallback {
		t.Fatalf("unexpected support flags = %#v", catalog)
	}
	if catalog.Skills[0].Scope != "global" || catalog.Skills[0].SourcePath == "" {
		t.Fatalf("skill descriptor = %#v", catalog.Skills[0])
	}
}

func TestProviderCapabilitiesUseProviderSpecificDirectorySemanticsAndFileMetadata(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(workspace, ".claude", "commands"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, ".claude", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, ".claude", "skills", "feature-design"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, ".claude", "skills", "feature-design", "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	commandContent := "title: Implementation Planner\ndescription: Build an implementation plan from the workspace context.\n"
	if err := os.WriteFile(filepath.Join(workspace, ".claude", "commands", "plan.yaml"), []byte(commandContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".claude", "agents", "review.md"), []byte("# Review Agent\nChecks assumptions and risks."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".claude", "skills", "feature-design", "SKILL.md"), []byte("# Feature Design\nDesign feature delivery flow."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".claude", "skills", "feature-design", "references", "checklist.md"), []byte("# Checklist\nShould not appear as a capability."), 0o644); err != nil {
		t.Fatal(err)
	}
	skills, agents := discoverProviderCapabilities("claude", workspace)
	if len(skills) != 2 || len(agents) != 1 {
		t.Fatalf("skills=%#v agents=%#v", skills, agents)
	}
	if skills[0].Kind != "skill" || skills[0].Name != "Feature Design" || skills[0].SourcePath != ".claude/skills/feature-design/SKILL.md" {
		t.Fatalf("skill = %#v", skills[0])
	}
	if skills[1].Kind != "skill" || skills[1].Name != "Implementation Planner" || !strings.Contains(skills[1].Description, "implementation plan") {
		t.Fatalf("skill = %#v", skills[1])
	}
	if agents[0].Kind != "agent" || agents[0].Name != "Review Agent" || agents[0].SourcePath != ".claude/agents/review.md" {
		t.Fatalf("agent = %#v", agents[0])
	}
}

func TestProviderCapabilitiesIgnoreNonCapabilityFilesInProviderRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude", "skills", "code-review", "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude", "commands"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", ".last-update-result.json"), []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "skills", "code-review", "SKILL.md"), []byte("# Code Review\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "skills", "code-review", "references", "checklists.md"), []byte("# Checklist\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "commands", "plan.yaml"), []byte("title: Planner\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := newTestService(t).ProviderCapabilities("claude", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Skills) != 2 {
		t.Fatalf("skills=%#v", catalog.Skills)
	}
	for _, skill := range catalog.Skills {
		if strings.Contains(skill.SourcePath, "references") || strings.Contains(skill.SourcePath, ".last-update-result") {
			t.Fatalf("unexpected capability included: %#v", skill)
		}
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
	legacy := Settings{
		DefaultProvider: "codex", DefaultTerminal: "wezterm",
		Providers: map[string]LaunchTemplate{"codex": {Enabled: true, Executable: "codex", Args: []string{"Read {contextFile} and follow its {intent} instructions for {identifier}."}}},
		Terminals: map[string]LaunchTemplate{"wezterm": {Enabled: true, Executable: "wezterm"}},
	}
	if _, err := service.store.Save(legacy); err != nil {
		t.Fatal(err)
	}
	settings, err := service.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if got := settings.Providers["codex"].Args; len(got) != 1 || strings.Contains(got[0], "intent") || strings.Contains(got[0], "contextFile") || !strings.Contains(got[0], "{itemPath}") || !strings.Contains(got[0], "{prompt}") {
		t.Fatalf("migrated args = %#v", got)
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	service := New(NewSettingsRepository(filepath.Join(t.TempDir(), "ai-settings.yaml")))
	service.goos = "darwin"
	return service
}
