package workstream

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/storage"
	"kode-stream/internal/system"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
)

func TestLoadBranchScansSnapshotWithoutCheckout(t *testing.T) {
	root := newWorkstreamGitRepo(t)
	writeWorkstreamGitFile(t, root, "plans/platform/PM-001/README.md", "# PM-001: Main\n")
	workstreamGitCommit(t, root, "main plan")
	workstreamGitRun(t, root, "switch", "-c", "feature")
	writeWorkstreamGitFile(t, root, "plans/platform/PM-013/README.md", "# PM-013: Snapshot\n")
	writeWorkstreamGitFile(t, root, "plans/platform/PM-013/plan.yaml", "plan:\n  status: review\n")
	workstreamGitCommit(t, root, "snapshot plan")
	workstreamGitRun(t, root, "switch", "main")

	dir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dir, "workspaces.yaml"), git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	idx := itemindex.New(filepath.Join(dir, "items.yaml"))
	service := New(reg, idx, scanner.New(git), git)

	result, err := service.LoadBranch(workspace.ID, models.WorkstreamBranchLoadInput{Branch: "feature", Force: true})
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

func TestLoadBranchRescansWorkingTreeWhenItemDirectoryIsDeleted(t *testing.T) {
	root := newWorkstreamGitRepo(t)
	itemPath := "plans/platform/PM-001"
	writeWorkstreamGitFile(t, root, itemPath+"/README.md", "# PM-001\n")
	workstreamGitCommit(t, root, "add plan")

	dir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dir, "workspaces.yaml"), git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	service := New(reg, itemindex.New(filepath.Join(dir, "items.yaml")), scanner.New(git), git)

	first, err := service.LoadBranch(workspace.ID, models.WorkstreamBranchLoadInput{Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ItemCount != 1 {
		t.Fatalf("initial item count = %d, want 1", first.ItemCount)
	}
	if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(itemPath))); err != nil {
		t.Fatal(err)
	}

	refreshed, err := service.LoadBranch(workspace.ID, models.WorkstreamBranchLoadInput{Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.ItemCount != 0 || len(refreshed.Items) != 0 {
		t.Fatalf("items after directory deletion = %#v, want none", refreshed.Items)
	}
}

func TestLoadBranchReusesUnchangedWorkingTreeScan(t *testing.T) {
	root := newWorkstreamGitRepo(t)
	writeWorkstreamGitFile(t, root, "plans/platform/PM-001/README.md", "# PM-001\n")
	workstreamGitCommit(t, root, "add plan")

	dir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dir, "workspaces.yaml"), git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	service := New(reg, itemindex.New(filepath.Join(dir, "items.yaml")), scanner.New(git), git)

	first, err := service.LoadBranch(workspace.ID, models.WorkstreamBranchLoadInput{Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.LoadBranch(workspace.ID, models.WorkstreamBranchLoadInput{Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !second.ScannedAt.Equal(first.ScannedAt) {
		t.Fatalf("unchanged working tree was rescanned: first=%s second=%s", first.ScannedAt, second.ScannedAt)
	}
	if second.ItemCount != first.ItemCount {
		t.Fatalf("cached item count = %d, want %d", second.ItemCount, first.ItemCount)
	}
}

func TestLoadBranchKeepsSameIdentifierContentSeparateInSQLite(t *testing.T) {
	root := newWorkstreamGitRepo(t)
	writeWorkstreamGitFile(t, root, "plans/platform/PM-001/README.md", "# PM-001: Main\n")
	workstreamGitCommit(t, root, "main plan")
	workstreamGitRun(t, root, "switch", "-c", "feature")
	writeWorkstreamGitFile(t, root, "plans/platform/PM-001/README.md", "# PM-001: Feature\n")
	workstreamGitCommit(t, root, "feature plan")
	workstreamGitRun(t, root, "switch", "main")

	dir := t.TempDir()
	git := gitadapter.New()
	paths := workstreamStoragePaths(dir)
	state, err := storage.OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, git, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	defer state.SQLStore.Close()
	workspace, err := state.Workspaces.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	service := New(state.Workspaces, state.Items, scanner.New(git), git)

	mainResult, err := service.LoadBranch(workspace.ID, models.WorkstreamBranchLoadInput{Branch: "main", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	featureResult, err := service.LoadBranch(workspace.ID, models.WorkstreamBranchLoadInput{Branch: "feature", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(mainResult.Items) != 1 || len(featureResult.Items) != 1 {
		t.Fatalf("main=%#v feature=%#v", mainResult.Items, featureResult.Items)
	}
	if mainResult.Items[0].Identifier != featureResult.Items[0].Identifier {
		t.Fatalf("identifiers differ: main=%q feature=%q", mainResult.Items[0].Identifier, featureResult.Items[0].Identifier)
	}
	if mainResult.Items[0].Title == featureResult.Items[0].Title {
		t.Fatalf("branch-specific titles were collapsed: main=%q feature=%q", mainResult.Items[0].Title, featureResult.Items[0].Title)
	}
	mainItems, err := state.Items.BranchItems(workspace.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	featureItems, err := state.Items.BranchItems(workspace.ID, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if mainItems[0].Title != mainResult.Items[0].Title || featureItems[0].Title != featureResult.Items[0].Title {
		t.Fatalf("stored branch items mismatch: main=%#v feature=%#v", mainItems, featureItems)
	}
}

func workstreamStoragePaths(dir string) system.Paths {
	return system.Paths{
		Dir:                dir,
		RegistryFile:       filepath.Join(dir, "workspaces.yaml"),
		PlanIndexFile:      filepath.Join(dir, "item-index.yaml"),
		SQLiteDatabaseFile: filepath.Join(dir, "kode-stream.db"),
		KnowledgeIndexFile: filepath.Join(dir, "knowledge-index.yaml"),
		AuditLogFile:       filepath.Join(dir, "audit-log.jsonl"),
		SavedFiltersFile:   filepath.Join(dir, "saved-filters.yaml"),
		RecentItemsFile:    filepath.Join(dir, "recent-items.yaml"),
		AISettingsFile:     filepath.Join(dir, "ai-settings.yaml"),
	}
}

func newWorkstreamGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	workstreamGitRun(t, root, "config", "user.name", "Kode Stream")
	workstreamGitRun(t, root, "config", "user.email", "kode-stream@example.test")
	return root
}

func writeWorkstreamGitFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func workstreamGitCommit(t *testing.T, root, message string) {
	t.Helper()
	workstreamGitRun(t, root, "add", ".")
	workstreamGitRun(t, root, "commit", "-m", message)
}

func workstreamGitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}
