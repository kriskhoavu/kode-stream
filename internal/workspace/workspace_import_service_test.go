package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"plan-manager/internal/common/models"
	gitadapter "plan-manager/internal/git"
	"plan-manager/internal/item/index"
	"plan-manager/internal/workspace/registry"
)

func TestPreviewImportValidatesCandidatesAndDetectsDuplicates(t *testing.T) {
	first := importGitRepo(t)
	second := importGitRepo(t)
	dataDir := t.TempDir()
	reg := registry.New(filepath.Join(dataDir, "workspaces.yaml"), gitadapter.New())
	if _, err := reg.Create(models.WorkspaceInput{Name: "Registered", Path: second, BaselineBranch: "main", Sources: []string{"plans"}}); err != nil {
		t.Fatal(err)
	}
	service := New(reg, itemindex.New(filepath.Join(dataDir, "items.yaml")), nil, nil)
	source := filepath.Join(t.TempDir(), "workspaces.yaml")
	contents := fmt.Sprintf(`
- id: ignored-source-id
  name: First
  path: %q
  baselineBranch: main
  registrationMode: remote_clone
  remoteUrl: https://example.com/first.git
  clonePathManaged: true
  sources: [plans]
- name: Duplicate
  path: %q
  baselineBranch: main
  sources: [plans]
- name: Existing
  path: %q
  baselineBranch: main
  sources: [plans]
- name: Invalid
  path: %q
  baselineBranch: missing
  sources: [plans]
`, first, first, second, first)
	if err := os.WriteFile(source, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	preview, err := service.PreviewImport(source)
	if err != nil {
		t.Fatal(err)
	}
	if preview.DestinationPath != reg.Path() || preview.SourceFingerprint == "" || len(preview.Candidates) != 4 {
		t.Fatalf("preview = %+v", preview)
	}
	if preview.Summary != (models.WorkspaceImportSummary{Valid: 1, Invalid: 1, Duplicate: 1, AlreadyRegistered: 1}) {
		t.Fatalf("summary = %+v", preview.Summary)
	}
	statuses := []string{"valid", "duplicate", "already_registered", "invalid"}
	for i, status := range statuses {
		candidate := preview.Candidates[i]
		if candidate.Status != status || candidate.Selected != (status == "valid") || candidate.CandidateKey == "" {
			t.Fatalf("candidate %d = %+v", i, candidate)
		}
	}
	if preview.Candidates[0].Workspace.RegistrationMode != models.WorkspaceRegistrationModeExisting {
		t.Fatalf("mode = %q", preview.Candidates[0].Workspace.RegistrationMode)
	}
	listed, err := reg.List()
	if err != nil || len(listed) != 1 {
		t.Fatalf("preview mutated registry: records=%+v err=%v", listed, err)
	}
}

func TestPreviewImportRejectsUnsafeOrRemovedSchema(t *testing.T) {
	service := New(registry.New(filepath.Join(t.TempDir(), "registry.yaml"), gitadapter.New()), itemindex.New(filepath.Join(t.TempDir(), "items.yaml")), nil, nil)
	tests := map[string]string{
		"unknown field": "- name: Test\n  path: /tmp/test\n  baselineBranch: main\n  sources: [plans]\n  planDirectories: [plans]\n",
		"alias":         "- &workspace\n  name: Test\n  path: /tmp/test\n  baselineBranch: main\n  sources: [plans]\n- *workspace\n",
		"documents":     "[]\n---\n[]\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspaces.yaml")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := service.PreviewImport(path); err == nil {
				t.Fatal("expected import source rejection")
			}
		})
	}
}

func TestPreviewImportEnforcesPathExtensionSizeAndEntryLimits(t *testing.T) {
	service := New(registry.New(filepath.Join(t.TempDir(), "registry.yaml"), gitadapter.New()), itemindex.New(filepath.Join(t.TempDir(), "items.yaml")), nil, nil)
	if _, err := service.PreviewImport("relative.yaml"); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative path error = %v", err)
	}
	wrongExtension := filepath.Join(t.TempDir(), "workspaces.txt")
	if err := os.WriteFile(wrongExtension, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewImport(wrongExtension); err == nil || !strings.Contains(err.Error(), "YAML") {
		t.Fatalf("extension error = %v", err)
	}
	oversized := filepath.Join(t.TempDir(), "workspaces.yaml")
	if err := os.WriteFile(oversized, make([]byte, maxWorkspaceImportBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewImport(oversized); err == nil || !strings.Contains(err.Error(), "1 MiB") {
		t.Fatalf("size error = %v", err)
	}
	many := filepath.Join(t.TempDir(), "workspaces.yaml")
	if err := os.WriteFile(many, []byte(strings.Repeat("- {}\n", maxWorkspaceImportEntries+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewImport(many); err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("entry error = %v", err)
	}
}

func TestExistingWorkspaceDeletionNeverRemovesDirectory(t *testing.T) {
	root := importGitRepo(t)
	dataDir := t.TempDir()
	reg := registry.New(filepath.Join(dataDir, "workspaces.yaml"), gitadapter.New())
	idx := itemindex.New(filepath.Join(dataDir, "items.yaml"))
	service := New(reg, idx, nil, nil)
	created, err := reg.Create(models.WorkspaceInput{Name: "Imported", Path: root, BaselineBranch: "main", Sources: []string{"plans"}, RegistrationMode: models.WorkspaceRegistrationModeExisting})
	if err != nil {
		t.Fatal(err)
	}
	if created.ClonePathManaged {
		t.Fatal("existing workspace must not be managed")
	}
	if err := service.Delete(created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("imported workspace directory was removed: %v", err)
	}
}

func importGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	commit := exec.Command("git", "-C", root, "commit", "--allow-empty", "-m", "init")
	commit.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	return root
}
