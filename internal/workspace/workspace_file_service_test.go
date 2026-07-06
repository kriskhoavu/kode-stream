package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"plan-manager/internal/common/models"
	"plan-manager/internal/filesystem/content"
	workspaceaccess "plan-manager/internal/workspace/files"
)

type fakeRegistry struct{ workspace models.WorkspaceConfig }

func (f fakeRegistry) Get(id string) (models.WorkspaceConfig, bool, error) {
	return f.workspace, id == f.workspace.ID, nil
}

func (f fakeRegistry) List() ([]models.WorkspaceConfig, error) {
	return []models.WorkspaceConfig{f.workspace}, nil
}

type fakeGit struct {
	diff     string
	reverted []string
}

func (f *fakeGit) Diff(_, _ string) (string, error) { return f.diff, nil }
func (f *fakeGit) RevertPaths(_ string, paths []string) error {
	f.reverted = append([]string(nil), paths...)
	return nil
}
func (f *fakeGit) PathStates(_, _ string) ([]models.WorkspacePathGitState, error) {
	return []models.WorkspacePathGitState{{Path: "plans/item.md", Status: models.GitChangeModified}}, nil
}

type fakeAudit struct{ events []models.AuditEvent }

func (f *fakeAudit) Append(event models.AuditEvent) (models.AuditEvent, error) {
	f.events = append(f.events, event)
	return event, nil
}

type fakeRefresher struct{ calls int }

func (f *fakeRefresher) RefreshWorkspace(models.WorkspaceConfig) (models.ScanResult, error) {
	f.calls++
	return models.ScanResult{}, nil
}

func TestSavePreservesPermissionsAuditsAndRefreshesSources(t *testing.T) {
	service, workspace, audit, refresher := newTestService(t)
	path := filepath.Join(workspace.Path, "plans", "item.md")
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Save(workspace.ID, models.WorkspaceFileSaveInput{
		Path: "plans/item.md", Content: "updated", ExpectedHash: fileaccess.ContentHash(current),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.File.Content != "updated" || !result.Refreshed || refresher.calls != 1 {
		t.Fatalf("unexpected save result: %#v, refresh calls %d", result, refresher.calls)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("permissions = %o, want 640", info.Mode().Perm())
	}
	if len(audit.events) != 1 || audit.events[0].Status != models.AuditStatusSuccess {
		t.Fatalf("audit events = %#v", audit.events)
	}
}

func TestSaveRejectsMissingAndStaleHashes(t *testing.T) {
	service, workspace, audit, _ := newTestService(t)
	for _, hash := range []string{"", "stale"} {
		_, err := service.Save(workspace.ID, models.WorkspaceFileSaveInput{Path: "plans/item.md", Content: "updated", ExpectedHash: hash})
		if err == nil {
			t.Fatalf("hash %q succeeded", hash)
		}
	}
	if len(audit.events) != 2 || audit.events[0].Status != models.AuditStatusFailed {
		t.Fatalf("failed saves were not audited: %#v", audit.events)
	}
}

func TestDiffAndRevertUseOneGuardedPath(t *testing.T) {
	service, workspace, audit, refresher := newTestService(t)
	git := service.git.(*fakeGit)
	git.diff = "diff"
	diff, err := service.Diff(workspace.ID, "plans/item.md")
	if err != nil || diff != "diff" {
		t.Fatalf("Diff() = %q, %v", diff, err)
	}
	result, err := service.Revert(workspace.ID, models.WorkspaceFileRevertInput{Path: "plans/item.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(git.reverted) != 1 || git.reverted[0] != "plans/item.md" || !result.Refreshed || refresher.calls != 1 {
		t.Fatalf("unexpected revert: %#v, %#v", result, git.reverted)
	}
	if len(audit.events) != 1 || audit.events[0].Operation != "workspace_file_revert" {
		t.Fatalf("revert audit = %#v", audit.events)
	}
}

func TestReadRejectsBinaryAndSaveAcceptsText(t *testing.T) {
	service, workspace, _, _ := newTestService(t)
	if _, err := service.Read(workspace.ID, "binary.bin"); !errors.Is(err, fileaccess.ErrUnsupportedContent) {
		t.Fatalf("binary read error = %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(workspace.Path, "notes.txt"))
	saved, err := service.Save(workspace.ID, models.WorkspaceFileSaveInput{Path: "notes.txt", Content: "new", ExpectedHash: fileaccess.ContentHash(data)})
	if err != nil || saved.File.Content != "new" || !saved.File.Editable {
		t.Fatalf("text save = %+v, %v", saved, err)
	}
}

func TestSearchAndPathStates(t *testing.T) {
	service, workspace, _, _ := newTestService(t)
	search, err := service.Search("item", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(search.Results) != 1 || search.Results[0].Path != "plans/item.md" {
		t.Fatalf("search = %#v", search)
	}
	states, err := service.PathStates(workspace.ID)
	if err != nil || len(states) != 1 || states[0].Status != models.GitChangeModified {
		t.Fatalf("states = %#v, %v", states, err)
	}
}

func TestCreateAndRenameAuditAndRefreshConfiguredSources(t *testing.T) {
	service, workspace, audit, refresher := newTestService(t)
	created, err := service.CreateFile(workspace.ID, models.WorkspaceFileCreateInput{ParentPath: "plans", Name: "new.txt", Content: "new"})
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := service.Rename(workspace.ID, models.WorkspacePathRenameInput{Path: "plans/new.txt", DestinationPath: "plans/renamed.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !created.Refreshed || !renamed.Refreshed || refresher.calls != 2 {
		t.Fatalf("results = %#v %#v, refresh calls = %d", created, renamed, refresher.calls)
	}
	if len(audit.events) != 2 || audit.events[1].Operation != "workspace_path_rename" || len(audit.events[1].Paths) != 2 {
		t.Fatalf("audit = %#v", audit.events)
	}
}

func TestBlockedMutationIsAuditedWithoutRefresh(t *testing.T) {
	service, workspace, audit, refresher := newTestService(t)
	_, err := service.CreateDirectory(workspace.ID, models.WorkspaceDirectoryCreateInput{ParentPath: "plans", Name: "../escape"})
	if !errors.Is(err, workspaceaccess.ErrInvalidName) {
		t.Fatalf("error = %v", err)
	}
	if len(audit.events) != 1 || audit.events[0].Status != models.AuditStatusBlocked || refresher.calls != 0 {
		t.Fatalf("audit = %#v, refresh calls = %d", audit.events, refresher.calls)
	}
}

func newTestService(t *testing.T) (*WorkspaceFileService, models.WorkspaceConfig, *fakeAudit, *fakeRefresher) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string][]byte{
		"plans/item.md": []byte("original"),
		"notes.txt":     []byte("notes"),
		"binary.bin":    {0, 1, 2},
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	workspace := models.WorkspaceConfig{ID: "ws", Path: root, Sources: []string{"plans"}}
	audit := &fakeAudit{}
	refresher := &fakeRefresher{}
	return NewWorkspaceFileService(fakeRegistry{workspace}, workspaceaccess.NewWithIgnoreChecker(nil), &fakeGit{}, audit, refresher), workspace, audit, refresher
}
