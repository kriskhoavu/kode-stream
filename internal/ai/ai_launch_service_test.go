package ai

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/audit"
	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
	"kode-stream/internal/item/index"
	"kode-stream/internal/workspace/registry"
)

type recordedProcess struct {
	name string
	args []string
	dir  string
}

type recordingRunner struct {
	processes []recordedProcess
	err       error
}

func (r *recordingRunner) Start(name string, args []string, dir string) error {
	r.processes = append(r.processes, recordedProcess{name: name, args: append([]string(nil), args...), dir: dir})
	return r.err
}

func TestLaunchPassesCardPathAndStartsProviderInWorkspace(t *testing.T) {
	service, item, workspace, runner, wrapperDir, auditStore := launchTestService(t, true)
	eligibility, err := service.Eligibility(item.ID)
	if err != nil || !eligibility.Editable || !eligibility.CardContextAvailable || len(eligibility.Missing) != 0 {
		t.Fatalf("eligibility=%#v err=%v", eligibility, err)
	}
	result, err := service.Launch(item.ID, LaunchInput{Provider: "test-ai", Terminal: "wezterm", ContextMode: "card_context"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Accepted || result.ContextMode != "card_context" || len(runner.processes) != 1 {
		t.Fatalf("result=%#v processes=%#v", result, runner.processes)
	}
	process := runner.processes[0]
	if process.dir != workspace.Path || len(process.args) < 5 || process.args[0] != "start" || process.args[1] != "--cwd" {
		t.Fatalf("process = %#v", process)
	}
	wrapperPath := process.args[len(process.args)-1]
	if !strings.HasPrefix(wrapperPath, wrapperDir) {
		t.Fatalf("wrapper path = %q", wrapperPath)
	}
	wrapperContent, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wrapperContent), item.ItemPath) || strings.Contains(string(wrapperContent), filepath.Join(workspace.Path, item.ItemPath)) {
		t.Fatalf("wrapper content = %s", string(wrapperContent))
	}
	if !strings.Contains(string(wrapperContent), "/verification-checkpoints") {
		t.Fatalf("wrapper missing checkpoint callback: %s", string(wrapperContent))
	}
	entries, err := os.ReadDir(wrapperDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("unexpected generated resources=%#v err=%v", entries, err)
	}
	events, err := auditStore.Recent(10)
	if err != nil || len(events) != 1 || events[0].Status != models.AuditStatusSuccess || len(events[0].Paths) != 0 {
		t.Fatalf("events=%#v err=%v", events, err)
	}
}

func TestCardContextDoesNotRequireStructuredPlan(t *testing.T) {
	service, item, _, runner, _, auditStore := launchTestService(t, false)
	eligibility, eligibilityErr := service.Eligibility(item.ID)
	if eligibilityErr != nil || !eligibility.CardContextAvailable || len(eligibility.Missing) != 0 {
		t.Fatalf("eligibility=%#v err=%v", eligibility, eligibilityErr)
	}
	result, err := service.Launch(item.ID, LaunchInput{Provider: "test-ai", Terminal: "wezterm", ContextMode: "card_context"})
	if err != nil || !result.Accepted || len(runner.processes) != 1 {
		t.Fatalf("result=%#v processes=%#v err=%v", result, runner.processes, err)
	}
	events, _ := auditStore.Recent(10)
	if len(events) != 1 || events[0].Status != models.AuditStatusSuccess {
		t.Fatalf("events = %#v", events)
	}
}

func TestLaunchRejectsSnapshotAndMissingTools(t *testing.T) {
	service, item, _, runner, _, _ := launchTestService(t, true)
	item.SourceMode = "snapshot"
	if err := service.launch.index.ReplaceWorkspace(item.WorkspaceID, []models.ItemDetail{item}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	_, err := service.Launch(item.ID, LaunchInput{Provider: "test-ai", Terminal: "wezterm", ContextMode: "card_context"})
	var launchErr *LaunchError
	if !errors.As(err, &launchErr) || launchErr.Code != "item_not_editable" || len(runner.processes) != 0 {
		t.Fatalf("err=%#v processes=%#v", err, runner.processes)
	}
}

func TestWorkspaceOnlyLaunchesWithoutCardContext(t *testing.T) {
	service, item, workspace, runner, wrapperDir, _ := launchTestService(t, true)
	item.SourceMode = "snapshot"
	item.Editable = false
	if err := service.launch.index.ReplaceWorkspace(item.WorkspaceID, []models.ItemDetail{item}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	result, err := service.Launch(item.ID, LaunchInput{Provider: "test-ai", Terminal: "wezterm", ContextMode: "workspace_only"})
	if err != nil || !result.Accepted || result.ContextMode != "workspace_only" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(runner.processes) != 1 {
		t.Fatalf("processes=%#v", runner.processes)
	}
	process := runner.processes[0]
	if process.dir != workspace.Path || len(process.args) != 5 || process.args[0] != "start" || process.args[4] == "" {
		t.Fatalf("process=%#v", process)
	}
	wrapperContent, err := os.ReadFile(process.args[4])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wrapperContent), item.ItemPath) {
		t.Fatalf("workspace-only wrapper unexpectedly includes item path: %s", string(wrapperContent))
	}
	entries, err := os.ReadDir(wrapperDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("workspace-only created resources=%#v err=%v", entries, err)
	}
}

func TestLaunchExpandsPresetPrompt(t *testing.T) {
	service, item, _, runner, _, _ := launchTestService(t, true)
	result, err := service.Launch(item.ID, LaunchInput{Provider: "test-ai", Terminal: "wezterm", ContextMode: "card_context", PresetID: "implementation-plan"})
	if err != nil || !result.Accepted || result.PresetID != "implementation-plan" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(runner.processes) != 1 {
		t.Fatalf("processes=%#v", runner.processes)
	}
	data, readErr := os.ReadFile(runner.processes[0].args[len(runner.processes[0].args)-1])
	if readErr != nil || !strings.Contains(string(data), "Create or update the implementation plan") {
		t.Fatalf("wrapper=%q err=%v", string(data), readErr)
	}
}

func TestLaunchExpandsFreePromptAndRejectsInvalidPromptInput(t *testing.T) {
	service, item, _, runner, _, _ := launchTestService(t, true)
	result, err := service.Launch(item.ID, LaunchInput{Provider: "test-ai", Terminal: "wezterm", ContextMode: "card_context", CustomPrompt: "Use the Jira context first."})
	if err != nil || !result.Accepted {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(runner.processes) != 1 {
		t.Fatalf("processes=%#v", runner.processes)
	}
	data, readErr := os.ReadFile(runner.processes[0].args[len(runner.processes[0].args)-1])
	if readErr != nil || !strings.Contains(string(data), "Use the Jira context first.") {
		t.Fatalf("wrapper=%q err=%v", string(data), readErr)
	}
	_, err = service.Launch(item.ID, LaunchInput{Provider: "test-ai", Terminal: "wezterm", ContextMode: "card_context", PresetID: "missing"})
	var launchErr *LaunchError
	if !errors.As(err, &launchErr) || launchErr.Code != "invalid_prompt" {
		t.Fatalf("err=%#v", err)
	}
	_, err = service.Launch(item.ID, LaunchInput{Provider: "test-ai", Terminal: "wezterm", ContextMode: "card_context", PresetID: "implementation-plan", CustomPrompt: "also"})
	if !errors.As(err, &launchErr) || launchErr.Code != "invalid_prompt" {
		t.Fatalf("err=%#v", err)
	}
}

func TestLaunchUsesPromptDraftAndCapabilitySelections(t *testing.T) {
	service, item, _, runner, _, _ := launchTestService(t, true)
	catalog, err := service.ProviderCapabilities("test-ai", item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Skills) == 0 || len(catalog.Agents) == 0 {
		t.Fatalf("catalog = %#v", catalog)
	}
	result, err := service.Launch(item.ID, LaunchInput{
		Provider:       "test-ai",
		Terminal:       "wezterm",
		ContextMode:    "workspace_only",
		PresetID:       "implementation-plan",
		PromptDraft:    "Use the preset but add rollout notes.",
		SelectedSkills: []string{catalog.Skills[0].ID},
		SelectedAgents: []string{catalog.Agents[0].ID},
	})
	if err != nil || !result.Accepted || result.PresetID != "implementation-plan" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(runner.processes) != 1 {
		t.Fatalf("processes=%#v", runner.processes)
	}
	data, readErr := os.ReadFile(runner.processes[0].args[len(runner.processes[0].args)-1])
	if readErr != nil {
		t.Fatal(readErr)
	}
	content := string(data)
	if !strings.Contains(content, "Use the preset but add rollout notes.") || !strings.Contains(content, catalog.Skills[0].Name) || !strings.Contains(content, catalog.Agents[0].Name) {
		t.Fatalf("wrapper=%q", content)
	}
}

func TestLaunchFailureIsAuditedAsFailed(t *testing.T) {
	service, item, _, runner, _, auditStore := launchTestService(t, true)
	runner.err = errors.New("terminal refused launch")
	_, err := service.Launch(item.ID, LaunchInput{Provider: "test-ai", Terminal: "wezterm", ContextMode: "card_context"})
	var launchErr *LaunchError
	if !errors.As(err, &launchErr) || launchErr.Code != "launch_failed" {
		t.Fatalf("err = %#v", err)
	}
	events, readErr := auditStore.Recent(10)
	if readErr != nil || len(events) != 1 || events[0].Status != models.AuditStatusFailed {
		t.Fatalf("events=%#v err=%v", events, readErr)
	}
}

func TestShellQuoteKeepsCommandTextLiteral(t *testing.T) {
	value := `a' b; $(touch unsafe)`
	quoted := shellQuote(value)
	if quoted != `'a'"'"' b; $(touch unsafe)'` {
		t.Fatalf("quoted = %q", quoted)
	}
}

func launchTestService(t *testing.T, structured bool) (*Service, models.ItemDetail, models.WorkspaceConfig, *recordingRunner, string, *audit.Store) {
	t.Helper()
	root := t.TempDir()
	planDir := filepath.Join(root, "plans", "platform", "PM-018")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skills", "implementation-planning.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if structured {
		if err := os.WriteFile(filepath.Join(planDir, "plan.yaml"), []byte("plan:\n  status: draft\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(planDir, "implementation-plan.md"), []byte("# Plan\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitCommand(t, root, "init", "-b", "main")
	gitCommand(t, root, "add", ".")
	commit := exec.Command("git", "-C", root, "commit", "--allow-empty", "-m", "seed")
	commit.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	dataDir := t.TempDir()
	reg := registry.New(filepath.Join(dataDir, "workspaces.yaml"), gitadapter.New())
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Test", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{
		ID: "item-pm-018", WorkspaceID: workspace.ID, WorkspaceName: workspace.Name,
		Branch: "main", SourceMode: "working_tree", Editable: true, Scope: "platform",
		Identifier: "PM-018", Title: "External AI", ItemPath: "plans/platform/PM-018",
	}, Documents: []models.ItemDocument{{Path: "implementation-plan.md", Label: "Implementation Plan"}}}
	index := itemindex.New(filepath.Join(dataDir, "item-index.yaml"))
	if err := index.ReplaceWorkspace(workspace.ID, []models.ItemDetail{item}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(dataDir, "tool")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	store := NewSettingsRepository(filepath.Join(dataDir, "ai-settings.yaml"))
	_, err = store.Save(Settings{
		DefaultProvider: "test-ai", DefaultTerminal: "wezterm",
		Providers: map[string]LaunchTemplate{"test-ai": {Enabled: true, Executable: executable, Args: []string{"Read {contextFile}", "{contextMode}", "{identifier}", "{prompt}"}}},
		Terminals: map[string]LaunchTemplate{"wezterm": {Enabled: true, Executable: executable}},
	})
	if err != nil {
		t.Fatal(err)
	}
	auditStore := audit.New(filepath.Join(dataDir, "audit.jsonl"))
	runner := &recordingRunner{}
	wrapperDir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".agents", "reviewer.md"), []byte("# agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := New(store).ConfigureLaunch(reg, index, auditStore, wrapperDir)
	service.goos = "darwin"
	service.launch.runner = runner
	service.launch.now = func() time.Time { return time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC) }
	return service, item, workspace, runner, wrapperDir, auditStore
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if strings.Contains(value, target) {
			return true
		}
	}
	return false
}

func gitCommand(t *testing.T, root string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	if output, err := exec.Command("git", commandArgs...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
