package gitworktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kode-stream/internal/common/models"
	appgit "kode-stream/internal/git"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestAdapterCreatesWorktreesUnderTheRootOnHarnessBranches(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	git(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "a.md"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "base")
	adapter := New(filepath.Join(t.TempDir(), "worktrees"), appgit.New())
	workspace := models.WorkspaceConfig{ID: "ws", Path: repo, BaselineBranch: "main"}
	if _, err := adapter.Create(ctx, workspace, "run-1", "feature/x"); err == nil {
		t.Fatal("a non-harness branch was accepted")
	}
	if _, err := adapter.Create(ctx, workspace, "../escape", "harness/x"); err == nil {
		t.Fatal("an escaping run id was accepted")
	}
	worktree, err := adapter.Create(ctx, workspace, "run-1", "harness/PM-1-abc")
	if err != nil {
		t.Fatal(err)
	}
	if worktree.StartPoint != "main" || worktree.BaseCommit == "" {
		t.Fatalf("worktree = %#v", worktree)
	}
	if err := os.WriteFile(filepath.Join(worktree.Path, "b.md"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, stat, err := adapter.Diff(ctx, worktree.Path, worktree.BaseCommit)
	if err != nil || len(files) != 1 || files[0].Path != "b.md" || stat == "" {
		t.Fatalf("files = %#v stat=%q err=%v", files, stat, err)
	}
	commit, err := adapter.Commit(ctx, worktree.Path, "PM-1: Add b")
	if err != nil || commit == "" {
		t.Fatalf("commit = %q err=%v", commit, err)
	}
	if err := adapter.Push(ctx, worktree.Path, "main"); err == nil {
		t.Fatal("pushing a non-harness branch was accepted")
	}
	if _, _, err := adapter.Diff(ctx, repo, worktree.BaseCommit); err == nil {
		t.Fatal("a diff outside the root was accepted")
	}
	if err := adapter.Remove(ctx, workspace, worktree.Path, worktree.Branch); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree.Path); !os.IsNotExist(err) {
		t.Fatal("worktree still exists")
	}
}
