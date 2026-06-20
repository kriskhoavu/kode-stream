package workspacefiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"plan-manager/internal/fileaccess"
	"plan-manager/internal/models"
	workspaceaccess "plan-manager/internal/workspacefiles"
)

type fakeRegistry struct{ workspace models.WorkspaceConfig }

func (f fakeRegistry) Get(id string) (models.WorkspaceConfig, bool, error) {
	return f.workspace, id == f.workspace.ID, nil
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

func TestReadRejectsBinaryAndSaveRejectsNonMarkdown(t *testing.T) {
	service, workspace, _, _ := newTestService(t)
	if _, err := service.Read(workspace.ID, "binary.bin"); !errors.Is(err, fileaccess.ErrUnsupportedContent) {
		t.Fatalf("binary read error = %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(workspace.Path, "notes.txt"))
	_, err := service.Save(workspace.ID, models.WorkspaceFileSaveInput{Path: "notes.txt", Content: "new", ExpectedHash: fileaccess.ContentHash(data)})
	if !errors.Is(err, workspaceaccess.ErrMarkdownOnly) {
		t.Fatalf("text save error = %v", err)
	}
}

func newTestService(t *testing.T) (*Service, models.WorkspaceConfig, *fakeAudit, *fakeRefresher) {
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
	return New(fakeRegistry{workspace}, workspaceaccess.NewWithIgnoreChecker(nil), &fakeGit{}, audit, refresher), workspace, audit, refresher
}
