package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"plan-manager/internal/gitadapter"
	"plan-manager/internal/itemindex"
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
