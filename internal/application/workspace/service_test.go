package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"plan-manager/internal/fileaccess"
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/itemindex"
	"plan-manager/internal/itemwriter"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
	"plan-manager/internal/scanner"
)

func TestStateReflectsWorkspaceAndItemChanges(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "workspaces.yaml")
	indexPath := filepath.Join(dir, "item-index.yaml")
	reg := registry.New(registryPath, gitadapter.New())
	idx := itemindex.New(indexPath)
	service := New(reg, idx, nil, nil)

	first, err := service.State()
	if err != nil {
		t.Fatal(err)
	}
	if first.WorkspaceCount != 0 || first.ItemCount != 0 {
		t.Fatalf("unexpected empty state: %+v", first)
	}

	updatedAt := time.Date(2026, 6, 20, 1, 2, 3, 0, time.UTC)
	if err := idx.ReplaceWorkspace("workspace-1", []models.ItemDetail{{
		ItemSummary: models.ItemSummary{
			ID:             "item-1",
			WorkspaceID:    "workspace-1",
			WorkspaceName:  "Workspace",
			Branch:         "main",
			Scope:          "platform",
			Identifier:     "PM-003",
			Title:          "Architecture",
			Status:         models.StatusDraft,
			UpdatedAt:      updatedAt,
			MetadataSource: "plan.yaml",
		},
	}}, nil, updatedAt); err != nil {
		t.Fatal(err)
	}

	next, err := service.State()
	if err != nil {
		t.Fatal(err)
	}
	if next.ItemCount != 1 {
		t.Fatalf("item count = %d, want 1", next.ItemCount)
	}
	if next.Version == first.Version {
		t.Fatal("state version should change when indexed items change")
	}
}

func TestNonNilWarningsReturnsEmptySlice(t *testing.T) {
	if got := NonNilWarnings(nil); got == nil || len(got) != 0 {
		t.Fatalf("NonNilWarnings(nil) = %#v", got)
	}
}

func TestLoadBranchScansSnapshotWithoutCheckout(t *testing.T) {
	root := newWorkspaceGitRepo(t)
	writeWorkspaceGitFile(t, root, "plans/platform/PM-001/README.md", "# PM-001: Main\n")
	workspaceGitCommit(t, root, "main plan")
	workspaceGitRun(t, root, "switch", "-c", "feature")
	writeWorkspaceGitFile(t, root, "plans/platform/PM-013/README.md", "# PM-013: Snapshot\n")
	writeWorkspaceGitFile(t, root, "plans/platform/PM-013/plan.yaml", "plan:\n  status: review\n")
	workspaceGitCommit(t, root, "snapshot plan")
	workspaceGitRun(t, root, "switch", "main")

	dir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dir, "workspaces.yaml"), git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	idx := itemindex.New(filepath.Join(dir, "items.yaml"))
	service := New(reg, idx, scanner.New(git), nil, git)

	result, err := service.LoadBranch(workspace.ID, models.BranchLoadInput{Branch: "feature", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.SourceMode != "snapshot" || result.CurrentCheckoutBranch != "main" || result.Branch != "feature" || result.ItemCount != 2 {
		t.Fatalf("branch result = %+v", result)
	}
	current, err := git.CurrentBranch(root)
	if err != nil {
		t.Fatal(err)
	}
	if current != "main" {
		t.Fatalf("branch load checked out %q", current)
	}
	if result.Items[0].SourceMode != "snapshot" || result.Items[0].Editable {
		t.Fatalf("snapshot item metadata = %+v", result.Items[0])
	}
}

func TestSourceStructureIncludesProposalsAndPreview(t *testing.T) {
	root := newWorkspaceGitRepo(t)
	writeWorkspaceGitFile(t, root, "docs/api/feature/DI-101/README.md", "# DI-101: API Search\n")
	workspaceGitCommit(t, root, "docs")
	dir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dir, "workspaces.yaml"), git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"docs"}})
	if err != nil {
		t.Fatal(err)
	}
	service := New(reg, itemindex.New(filepath.Join(dir, "items.yaml")), scanner.New(git), nil, git)

	result, err := service.SourceStructure(workspace.ID, "docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Proposals) == 0 || result.Proposals[0].ID != "actual-folder-feature-item" {
		t.Fatalf("unexpected proposals: %#v", result.Proposals)
	}
	if len(result.Preview) != 1 || result.Preview[0].Source != "docs" || result.Preview[0].Item != "DI-101" || result.Preview[0].Title != "API Search" {
		t.Fatalf("unexpected preview: %#v", result.Preview)
	}
}

func TestResetSourceStructureRemovesSettingsAndRescans(t *testing.T) {
	root := newWorkspaceGitRepo(t)
	writeWorkspaceGitFile(t, root, "docs/workspace-settings.yaml", `version: 1
cards:
  - pathPattern: "{scope}/feature/{identifier}"
    fields:
      scope: "{scope}"
      identifier: "{identifier}"
      title: readme_heading
      status: draft
      tags: [docs]
`)
	writeWorkspaceGitFile(t, root, "docs/api/feature/DI-101/README.md", "# DI-101: API Search\n")
	workspaceGitCommit(t, root, "configured docs")
	dir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dir, "workspaces.yaml"), git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"docs"}})
	if err != nil {
		t.Fatal(err)
	}
	idx := itemindex.New(filepath.Join(dir, "items.yaml"))
	scan := scanner.New(git)
	writer := itemwriter.New(fileaccess.New(), scan, idx, reg)
	service := New(reg, idx, scan, writer, git)

	result, err := service.ResetSourceStructure(workspace.ID, "docs")
	if err != nil {
		t.Fatal(err)
	}
	if result.Exists {
		t.Fatalf("expected reset result to report no settings file: %+v", result.SourceSettingsResult)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "workspace-settings.yaml")); !os.IsNotExist(err) {
		t.Fatalf("settings file still exists or stat failed unexpectedly: %v", err)
	}
	if result.Scan.ItemCount != 1 {
		t.Fatalf("expected scan after reset, got %+v", result.Scan)
	}
}

func newWorkspaceGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	workspaceGitRun(t, root, "config", "user.name", "Plan Manager")
	workspaceGitRun(t, root, "config", "user.email", "plan-manager@example.test")
	return root
}

func writeWorkspaceGitFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func workspaceGitCommit(t *testing.T, root, message string) {
	t.Helper()
	workspaceGitRun(t, root, "add", ".")
	workspaceGitRun(t, root, "commit", "-m", message)
}

func workspaceGitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}
