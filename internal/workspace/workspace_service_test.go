package workspace

// Workspace service contract tests.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/content"
	gitadapter "kode-stream/internal/git"
	"kode-stream/internal/item/index"
	"kode-stream/internal/item/writer"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
)

type localClonePort struct{ source string }

func (c localClonePort) CloneWithProgress(_ string, destination string, progress func(string)) error {
	output, err := exec.Command("git", "clone", c.source, destination).CombinedOutput()
	if progress != nil {
		progress(string(output))
	}
	if err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	return nil
}

func TestStateReflectsWorkspaceAndItemChanges(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "workspaces.yaml")
	indexPath := filepath.Join(dir, "item-index.yaml")
	reg := registry.New(registryPath, gitadapter.New())
	idx := itemindex.New(indexPath)
	service := New(ServiceDependencies{Registry: reg, Index: idx})

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
	service := New(ServiceDependencies{Registry: reg, Index: itemindex.New(filepath.Join(dir, "items.yaml")), Scanner: scanner.New(git), Cloner: git})

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

func TestSourceStructureSaveRevalidatesSymlinkBoundaryAtUseTime(t *testing.T) {
	root := newWorkspaceGitRepo(t)
	writeWorkspaceGitFile(t, root, "docs/item/README.md", "# Item\n")
	workspaceGitCommit(t, root, "add source")
	dataDir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dataDir, "workspaces.yaml"), git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"docs"}})
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.RemoveAll(filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	idx := itemindex.New(filepath.Join(dataDir, "items.yaml"))
	scan := scanner.New(git)
	service := New(ServiceDependencies{Registry: reg, Index: idx, Scanner: scan, Writer: itemwriter.New(fileaccess.New(), scan, idx, reg), Cloner: git})
	_, err = service.SaveSourceStructure(workspace.ID, "docs", scanner.DefaultSourceStructureSettings("docs"))
	if err == nil || !strings.Contains(err.Error(), "boundary") {
		t.Fatalf("error=%v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, scanner.SourceStructureSettingsFile)); !os.IsNotExist(statErr) {
		t.Fatalf("outside settings were written: %v", statErr)
	}
}

func TestCreateRemoteCloneWorkspace(t *testing.T) {
	remote := newWorkspaceGitRepo(t)
	writeWorkspaceGitFile(t, remote, "plans/platform/PM-101/README.md", "# PM-101\n")
	workspaceGitCommit(t, remote, "seed remote")

	cloneRoot := t.TempDir()
	dir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dir, "workspaces.yaml"), git)
	service := New(ServiceDependencies{Registry: reg, Index: itemindex.New(filepath.Join(dir, "items.yaml")), Scanner: scanner.New(git), Cloner: localClonePort{source: remote}})

	workspace, err := service.Create(models.WorkspaceInput{
		Name:             "Remote Workspace",
		RegistrationMode: models.WorkspaceRegistrationModeRemoteClone,
		RemoteURL:        "https://example.com/org/remote.git",
		CloneRoot:        cloneRoot,
		BaselineBranch:   "main",
		Sources:          []string{"plans"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if workspace.RegistrationMode != models.WorkspaceRegistrationModeRemoteClone || workspace.RemoteURL != "https://example.com/org/remote.git" || !workspace.ClonePathManaged || !workspace.ManagedCloneVerified {
		t.Fatalf("workspace mode metadata = %+v", workspace)
	}
	resolvedCloneRoot, _ := filepath.EvalSymlinks(cloneRoot)
	resolvedWorkspacePath, _ := filepath.EvalSymlinks(workspace.Path)
	if workspace.Path == remote || !strings.HasPrefix(resolvedWorkspacePath, resolvedCloneRoot) {
		t.Fatalf("workspace path = %q (%q), remote = %q, cloneRoot = %q (%q)", workspace.Path, resolvedWorkspacePath, remote, cloneRoot, resolvedCloneRoot)
	}
	if _, err := os.Stat(filepath.Join(workspace.Path, ".git")); err != nil {
		t.Fatalf("expected clone to include .git directory: %v", err)
	}
	if err := service.Delete(workspace.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(workspace.Path); !os.IsNotExist(err) {
		t.Fatalf("managed clone was not deleted: %v", err)
	}
}

func TestCreateRemoteCloneWorkspaceRejectsInvalidURL(t *testing.T) {
	dir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dir, "workspaces.yaml"), git)
	service := New(ServiceDependencies{Registry: reg, Index: itemindex.New(filepath.Join(dir, "items.yaml")), Scanner: scanner.New(git), Cloner: git})

	_, err := service.Create(models.WorkspaceInput{
		Name:             "Remote Workspace",
		RegistrationMode: models.WorkspaceRegistrationModeRemoteClone,
		RemoteURL:        "not-a-url",
		CloneRoot:        t.TempDir(),
		BaselineBranch:   "main",
		Sources:          []string{"plans"},
	})
	if err == nil || !strings.Contains(err.Error(), "remote URL") {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteRefusesLegacyManagedCloneWithoutOwnershipProof(t *testing.T) {
	managedRoot := t.TempDir()
	managedRepo := filepath.Join(managedRoot, "managed-clone")
	if output, err := exec.Command("git", "init", "-b", "main", managedRepo).CombinedOutput(); err != nil {
		t.Fatalf("git init managed repo: %v: %s", err, output)
	}
	if err := os.MkdirAll(filepath.Join(managedRepo, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", managedRepo, "add", ".").CombinedOutput(); err != nil {
		t.Fatalf("git add managed repo: %v: %s", err, output)
	}
	commit := exec.Command("git", "-C", managedRepo, "commit", "--allow-empty", "-m", "init")
	commit.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit managed repo: %v: %s", err, output)
	}

	root := t.TempDir()
	git := gitadapter.New()
	registryPath := filepath.Join(root, "workspaces.yaml")
	reg := registry.New(registryPath, git)
	workspace, err := reg.Create(models.WorkspaceInput{
		Name:             "Managed",
		Path:             managedRepo,
		RegistrationMode: models.WorkspaceRegistrationModeRemoteClone,
		RemoteURL:        "https://example.com/org/repo.git",
		BaselineBranch:   "main",
		Sources:          []string{"plans"},
	})
	if err != nil {
		t.Fatal(err)
	}
	workspace.ClonePathManaged = true
	legacyData, err := yaml.Marshal([]models.WorkspaceConfig{workspace})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registryPath, legacyData, 0o600); err != nil {
		t.Fatal(err)
	}
	reg = registry.New(registryPath, git)
	idx := itemindex.New(filepath.Join(root, "item-index.yaml"))
	service := New(ServiceDependencies{Registry: reg, Index: idx, Scanner: scanner.New(git), Cloner: git})

	if err := service.Delete(workspace.ID); err == nil {
		t.Fatal("expected unproven managed clone deletion to be refused")
	}
	if _, err := os.Stat(managedRepo); err != nil {
		t.Fatalf("legacy path must remain: %v", err)
	}
}

func TestFileWorkspaceDeleteRegistryFailureLeavesRecoverableRegisteredWorkspace(t *testing.T) {
	repositoryPath := t.TempDir()
	if output, err := exec.Command("git", "init", "-b", "main", repositoryPath).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.MkdirAll(filepath.Join(repositoryPath, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repositoryPath, "plans", "README.md"), []byte("# Plans\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", repositoryPath, "add", ".").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	commit := exec.Command("git", "-C", repositoryPath, "commit", "--allow-empty", "-m", "init")
	commit.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	base := t.TempDir()
	stateDir := filepath.Join(base, "state")
	registryPath := filepath.Join(stateDir, "workspaces.yaml")
	git := gitadapter.New()
	reg := registry.New(registryPath, git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Recoverable", Path: repositoryPath, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	idx := itemindex.New(filepath.Join(base, "item-index.yaml"))
	if err := idx.ReplaceWorkspace(workspace.ID, []models.ItemDetail{{ItemSummary: models.ItemSummary{ID: "item", WorkspaceID: workspace.ID, Title: "Item"}}}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	service := New(ServiceDependencies{Registry: reg, Index: idx, Cloner: git})
	backupDir := filepath.Join(base, "state-backup")
	if err := os.Rename(stateDir, backupDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stateDir, []byte("block registry directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(workspace.ID); err == nil {
		t.Fatal("expected registry persistence failure")
	}
	if _, ok, err := reg.Get(workspace.ID); err != nil || !ok {
		t.Fatalf("registered recovery state=%v err=%v", ok, err)
	}
	items, err := idx.Query(itemindex.Query{WorkspaceID: workspace.ID})
	if err != nil || len(items) != 0 {
		t.Fatalf("derived index should be safely rebuildable: %#v %v", items, err)
	}
	if err := os.Remove(stateDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backupDir, stateDir); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := registry.New(registryPath, git).Get(workspace.ID); err != nil || !ok {
		t.Fatalf("persisted registry recovery state=%v err=%v", ok, err)
	}
}

func TestResetSourceStructureRemovesSettingsAndRescans(t *testing.T) {
	root := newWorkspaceGitRepo(t)
	writeWorkspaceGitFile(t, root, "docs/workspace-settings.yaml", `version: 1
cards:
  - pathPattern: "{scope}/feature/{identifier}"
    fields:
      source: "{scope}"
      item: "{identifier}"
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
	service := New(ServiceDependencies{Registry: reg, Index: idx, Scanner: scan, Writer: writer, Cloner: git})

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
	workspaceGitRun(t, root, "config", "user.name", "Kode Stream")
	workspaceGitRun(t, root, "config", "user.email", "kode-stream@example.test")
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
