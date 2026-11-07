package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeLifecycleDiffRestoreAndCommit(t *testing.T) {
	ctx := context.Background()
	root := newGitRepo(t)
	writeGitFile(t, root, "keep.md", "base\n")
	writeGitFile(t, root, "edit.md", "base\n")
	gitCommit(t, root, "base")
	adapter := New()
	base, err := adapter.RevParseContext(ctx, root, "HEAD")
	if err != nil || len(base) < 7 {
		t.Fatalf("base = %q err=%v", base, err)
	}
	if adapter.HasRemoteContext(ctx, root, "origin") {
		t.Fatal("a fresh repository has no origin")
	}
	worktree := filepath.Join(t.TempDir(), "runs", "run-1")
	if err := adapter.WorktreeAddContext(ctx, root, worktree, "harness/PM-1-abc", base); err != nil {
		t.Fatal(err)
	}
	if err := adapter.ExcludeContext(ctx, root, ".harness/"); err != nil {
		t.Fatal(err)
	}
	if err := adapter.ExcludeContext(ctx, root, ".harness/"); err != nil {
		t.Fatal(err)
	}
	writeGitFile(t, worktree, "edit.md", "changed\nmore\n")
	writeGitFile(t, worktree, "new.md", "new\n")
	writeGitFile(t, worktree, "outside/scope.md", "nope\n")
	writeGitFile(t, worktree, ".harness/report.json", "{}")
	files, err := adapter.DiffFilesContext(ctx, worktree, base)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]DiffFile{}
	for _, file := range files {
		byPath[file.Path] = file
	}
	if len(files) != 3 || byPath["new.md"].Status != "added" || byPath["edit.md"].Additions != 2 || byPath["edit.md"].Deletions != 1 {
		t.Fatalf("files = %#v", files)
	}
	stat, err := adapter.DiffStatContext(ctx, worktree, base)
	if err != nil || !strings.Contains(stat, "3 files changed") {
		t.Fatalf("stat = %q err=%v", stat, err)
	}
	if err := adapter.RestorePathsContext(ctx, worktree, base, []string{"outside/scope.md", "edit.md"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(worktree, "outside", "scope.md")); !os.IsNotExist(err) {
		t.Fatalf("out-of-scope file kept: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(worktree, "edit.md")); string(data) != "base\n" {
		t.Fatalf("edit.md = %q", data)
	}
	if err := adapter.RestorePathsContext(ctx, worktree, base, []string{"../escape"}); err == nil {
		t.Fatal("an escaping path was restored")
	}
	commit, err := adapter.CommitAllContext(ctx, worktree, "PM-1: Add new")
	if err != nil || commit == "" || commit == base {
		t.Fatalf("commit = %q err=%v", commit, err)
	}
	again, err := adapter.CommitAllContext(ctx, worktree, "PM-1: Nothing")
	if err != nil || again != "" {
		t.Fatalf("empty commit = %q err=%v", again, err)
	}
	paths, err := adapter.ChangedPathsContext(ctx, worktree, base)
	if err != nil || len(paths) != 1 || paths[0] != "new.md" {
		t.Fatalf("paths = %v err=%v", paths, err)
	}
	if head, _ := adapter.RevParseContext(ctx, root, "HEAD"); head != base {
		t.Fatal("the workspace checkout moved")
	}
	if err := adapter.WorktreeRemoveContext(ctx, root, worktree, true); err != nil {
		t.Fatal(err)
	}
	if err := adapter.WorktreePruneContext(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := adapter.DeleteBranchContext(ctx, root, "harness/PM-1-abc"); err != nil {
		t.Fatal(err)
	}
}

func TestRefDiffAndShowFileReadABranchWithoutItsWorktree(t *testing.T) {
	ctx := context.Background()
	root := newGitRepo(t)
	writeGitFile(t, root, "plans/PM-1/README.md", "old\n")
	gitCommit(t, root, "base")
	adapter := New()
	base, _ := adapter.RevParseContext(ctx, root, "HEAD")
	worktree := filepath.Join(t.TempDir(), "runs", "run-1")
	if err := adapter.WorktreeAddContext(ctx, root, worktree, "harness/PM-1-abc", base); err != nil {
		t.Fatal(err)
	}
	writeGitFile(t, worktree, "plans/PM-1/README.md", "new plan\n")
	writeGitFile(t, worktree, "plans/PM-1/steps.md", "1. one\n")
	writeGitFile(t, worktree, "src/main.go", "package main\n")
	if _, err := adapter.CommitAllContext(ctx, worktree, "Plan PM-1"); err != nil {
		t.Fatal(err)
	}
	files, err := adapter.RefDiffFilesContext(ctx, root, base, "harness/PM-1-abc", "plans/")
	if err != nil || len(files) != 2 || files[0].Path != "plans/PM-1/README.md" || files[0].Status != "modified" || files[1].Status != "added" {
		t.Fatalf("files = %#v err=%v", files, err)
	}
	data, truncated, err := adapter.ShowFileContext(ctx, root, "harness/PM-1-abc", "plans/PM-1/README.md", 1024)
	if err != nil || truncated || string(data) != "new plan\n" {
		t.Fatalf("data = %q truncated=%v err=%v", data, truncated, err)
	}
	if _, truncated, err := adapter.ShowFileContext(ctx, root, "harness/PM-1-abc", "plans/PM-1/README.md", 3); err != nil || !truncated {
		t.Fatalf("limit: truncated=%v err=%v", truncated, err)
	}
	if _, _, err := adapter.ShowFileContext(ctx, root, "harness/PM-1-abc", "../secret", 1024); err == nil {
		t.Fatal("a path outside the repository was read")
	}
}
