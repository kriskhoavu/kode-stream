package git

// Git service contract tests.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kode-stream/internal/common/models"
)

type serviceRegistry struct{ workspace models.WorkspaceConfig }

func (r serviceRegistry) Get(id string) (models.WorkspaceConfig, bool, error) {
	return r.workspace, id == r.workspace.ID, nil
}

func TestServiceMutationsWaitForSharedWorkspaceLock(t *testing.T) {
	root := newGitRepo(t)
	writeGitFile(t, root, "plans/item.md", "seed")
	gitCommit(t, root, "seed")
	if err := os.WriteFile(filepath.Join(root, "plans/item.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := New()
	service := NewService(serviceRegistry{models.WorkspaceConfig{ID: "ws", Path: root, Sources: []string{"plans"}}}, nil, adapter)
	entered, release := make(chan struct{}), make(chan struct{})
	go func() {
		_ = adapter.WithWorkspaceMutation(root, func() error { close(entered); <-release; return nil })
	}()
	<-entered
	for _, run := range []func() models.GitOperationResult{
		func() models.GitOperationResult {
			return service.CommitContext(context.Background(), "ws", models.GitCommitInput{Message: "commit", Paths: []string{"plans/item.md"}})
		},
		func() models.GitOperationResult {
			return service.CreateBranchContext(context.Background(), "ws", models.BranchCreateInput{Name: "feature", Checkout: false})
		},
	} {
		done := make(chan models.GitOperationResult, 1)
		go func() { done <- run() }()
		select {
		case result := <-done:
			t.Fatalf("mutation escaped workspace lock: %#v", result)
		case <-time.After(30 * time.Millisecond):
		}
		close(release)
		if result := <-done; !result.OK {
			t.Fatalf("locked mutation failed: %#v", result)
		}
		entered, release = make(chan struct{}), make(chan struct{})
		go func() {
			_ = adapter.WithWorkspaceMutation(root, func() error { close(entered); <-release; return nil })
		}()
		<-entered
	}
	close(release)
}

func TestValidatePathsRequiresConfiguredSourcePaths(t *testing.T) {
	workspace := models.WorkspaceConfig{Sources: []string{"plans", "docs"}}
	if err := ValidatePaths(workspace, []string{"plans/platform/PM-003/README.md", "docs/guide.md"}); err != nil {
		t.Fatalf("expected valid paths: %v", err)
	}
}

func TestValidatePathsRejectsEmptyEscapedAndUnregisteredPaths(t *testing.T) {
	workspace := models.WorkspaceConfig{Sources: []string{"plans"}}
	for _, paths := range [][]string{
		{},
		{"../secret.md"},
		{"/tmp/secret.md"},
		{"src/main.go"},
	} {
		if err := ValidatePaths(workspace, paths); err == nil {
			t.Fatalf("expected %#v to be rejected", paths)
		}
	}
}

func TestNormalizeWorkspaceBranchesSortsDeduplicatesAndKeepsCurrent(t *testing.T) {
	result := normalizeWorkspaceBranches("workspace", "feature/current", []string{"main", "alpha", "main", " "})
	want := []string{"alpha", "feature/current", "main"}
	if result.WorkspaceID != "workspace" || result.Current != "feature/current" || len(result.Branches) != len(want) {
		t.Fatalf("branches = %#v", result)
	}
	for index := range want {
		if result.Branches[index] != want[index] {
			t.Fatalf("branch %d = %q, want %q", index, result.Branches[index], want[index])
		}
	}
}
