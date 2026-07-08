package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"kode-stream/internal/common/models"
)

func TestWritePathAllowsConfiguredSource(t *testing.T) {
	workspace := models.WorkspaceConfig{Path: t.TempDir(), Sources: []string{"plans"}}
	check := NewSafetyService().WritePath(workspace, "plans/platform/PM-004/README.md")
	if !check.OK {
		t.Fatalf("check = %#v, want allowed", check)
	}
}

func TestWritePathBlocksMissingWorkspaceAndEscapedPath(t *testing.T) {
	service := NewSafetyService()
	missing := models.WorkspaceConfig{Path: filepath.Join(t.TempDir(), "missing"), Sources: []string{"plans"}}
	if check := service.WritePath(missing, "plans/file.md"); check.OK || check.RecoveryHint == "" {
		t.Fatalf("missing workspace check = %#v", check)
	}
	workspace := models.WorkspaceConfig{Path: t.TempDir(), Sources: []string{"plans"}}
	if check := service.WritePath(workspace, "../secret.md"); check.OK || check.RecoveryHint == "" {
		t.Fatalf("escaped path check = %#v", check)
	}
}

func TestWorkspaceBlocksReadOnlyDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })
	if check := NewSafetyService().Workspace(models.WorkspaceConfig{Path: root}); check.OK {
		t.Fatalf("check = %#v, want blocked", check)
	}
}

func TestGitBlocksConflictsAndUnconfirmedDirtyTree(t *testing.T) {
	service := NewSafetyService()
	if check := service.Git(models.GitStatus{Conflicted: true}, true, "pull"); check.OK {
		t.Fatalf("conflict check = %#v, want blocked", check)
	}
	if check := service.Git(models.GitStatus{Dirty: true}, false, "switch branches"); check.OK || check.RecoveryHint == "" {
		t.Fatalf("dirty check = %#v, want blocked with hint", check)
	}
	if check := service.Git(models.GitStatus{Dirty: true}, true, "pull"); !check.OK {
		t.Fatalf("confirmed check = %#v, want allowed", check)
	}
}
