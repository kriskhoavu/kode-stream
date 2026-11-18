package gitworktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kode-stream/internal/common/models"
	appgit "kode-stream/internal/git"
	"kode-stream/internal/harness/ports"
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

func TestReadRunFilesReadsTheWorktreeThenTheBranch(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	git(t, repo, "init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(repo, "plans", "PM-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "plans", "PM-1", "README.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "base")
	adapter := New(filepath.Join(t.TempDir(), "worktrees"), appgit.New())
	workspace := models.WorkspaceConfig{ID: "ws", Path: repo, BaselineBranch: "main"}
	worktree, err := adapter.Create(ctx, workspace, "run-1", "harness/PM-1-abc")
	if err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(worktree.Path, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("plans/PM-1/README.md", "# The plan, not committed\n")
	write("plans/PM-1/steps.md", "1. Do it\n")
	write("plans/PM-1/blob.bin", "a\x00b")
	write("src/main.go", "package main\n")
	if err := os.Symlink("/etc/hosts", filepath.Join(worktree.Path, "plans", "PM-1", "link.md")); err != nil {
		t.Fatal(err)
	}

	files, from, err := adapter.ReadRunFiles(ctx, workspace, worktree.Path, worktree.BaseCommit, worktree.Branch, "plans/", 1024)
	if err != nil || from != "worktree" {
		t.Fatalf("from = %q err=%v", from, err)
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}
	if len(files) != 2 || byPath["plans/PM-1/README.md"] != "# The plan, not committed\n" || byPath["plans/PM-1/steps.md"] != "1. Do it\n" {
		t.Fatalf("worktree files = %#v", files)
	}
	if small, _, _ := adapter.ReadRunFiles(ctx, workspace, worktree.Path, worktree.BaseCommit, worktree.Branch, "plans/", 4); len(small) == 0 || !small[0].Truncated || small[0].Content != "" {
		t.Fatalf("limit = %#v", small)
	}

	if err := os.Remove(filepath.Join(worktree.Path, "plans", "PM-1", "link.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Commit(ctx, worktree.Path, "PM-1: Plan"); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Remove(ctx, workspace, worktree.Path, ""); err != nil {
		t.Fatal(err)
	}
	files, from, err = adapter.ReadRunFiles(ctx, workspace, "", worktree.BaseCommit, worktree.Branch, "plans/", 1024)
	if err != nil || from != "branch" || len(files) != 2 {
		t.Fatalf("branch files = %#v from=%q err=%v", files, from, err)
	}
	if none, from, err := adapter.ReadRunFiles(ctx, workspace, "", worktree.BaseCommit, "harness/gone", "plans/", 1024); err != nil || from != "" || len(none) != 0 {
		t.Fatalf("gone = %#v from=%q err=%v", none, from, err)
	}
	if none, from, _ := adapter.ReadRunFiles(ctx, workspace, "", worktree.BaseCommit, "main", "plans/", 1024); from != "" || len(none) != 0 {
		t.Fatalf("a non-harness branch was read: %#v", none)
	}
}

func TestRemoveDeletesTheBranchOfAWorktreeAlreadyGone(t *testing.T) {
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
	worktree, err := adapter.Create(ctx, workspace, "run-1", "harness/PM-1-abc")
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Remove(ctx, workspace, worktree.Path, ""); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Remove(ctx, workspace, worktree.Path, worktree.Branch); err != nil {
		t.Fatalf("the second remove failed: %v", err)
	}
	out, err := exec.Command("git", "-C", repo, "branch", "--list", worktree.Branch).Output()
	if err != nil || strings.TrimSpace(string(out)) != "" {
		t.Fatalf("branch still listed: %q err=%v", out, err)
	}
}

func TestPushNamesTheBranchAFeatureBranchAtTheRemote(t *testing.T) {
	ctx := context.Background()
	remote := t.TempDir()
	git(t, remote, "init", "-q", "--bare", "-b", "main")
	repo := t.TempDir()
	git(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "a.md"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "base")
	git(t, repo, "remote", "add", "origin", remote)
	git(t, repo, "push", "-q", "origin", "main")
	adapter := New(filepath.Join(t.TempDir(), "worktrees"), appgit.New())
	workspace := models.WorkspaceConfig{ID: "ws", Path: repo, BaselineBranch: "main"}
	worktree, err := adapter.Create(ctx, workspace, "run-1", "harness/APP-664-4b00e6")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree.Path, "b.md"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Commit(ctx, worktree.Path, "APP-664: Add b"); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Push(ctx, worktree.Path, worktree.Branch); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", remote, "branch", "--list").Output()
	if err != nil {
		t.Fatal(err)
	}
	if branches := strings.Fields(strings.ReplaceAll(string(out), "*", "")); strings.Join(branches, ",") != "feature/APP-664-4b00e6,main" {
		t.Fatalf("remote branches = %v", branches)
	}
	files, from, err := adapter.ReadRunFiles(ctx, workspace, "", worktree.BaseCommit, worktree.Branch, "", 1024)
	if err != nil || from != "branch" || len(files) != 1 {
		t.Fatalf("files = %#v from=%q err=%v", files, from, err)
	}
}

func TestChangedFilesAndFileDiffReadTheBranchOnceTheWorktreeIsGone(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	git(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "a.md"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "base")
	adapter := New(filepath.Join(t.TempDir(), "worktrees"), appgit.New())
	adapter.Remote = ""
	workspace := models.WorkspaceConfig{ID: "ws", Path: repo, BaselineBranch: "main"}
	worktree, err := adapter.Create(ctx, workspace, "run-1", "harness/PM-1-abc")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree.Path, "a.md"), []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree.Path, "new.md"), []byte("n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, from, err := adapter.ChangedFiles(ctx, workspace, worktree.Path, worktree.BaseCommit, worktree.Branch)
	if err != nil || from != "worktree" || len(files) != 2 {
		t.Fatalf("live files = %#v from=%q err=%v", files, from, err)
	}
	diff, _, from, err := adapter.FileDiff(ctx, workspace, worktree.Path, worktree.BaseCommit, worktree.Branch, "new.md", 1<<20)
	if err != nil || from != "worktree" || !strings.Contains(diff, "+n") {
		t.Fatalf("live diff = %q from=%q err=%v", diff, from, err)
	}
	if _, err := adapter.Commit(ctx, worktree.Path, "PM-1: Change a"); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "worktree", "remove", "--force", worktree.Path)
	files, from, err = adapter.ChangedFiles(ctx, workspace, worktree.Path, worktree.BaseCommit, worktree.Branch)
	if err != nil || from != "branch" || len(files) != 2 || files[0].Path != "a.md" || files[0].Additions != 1 {
		t.Fatalf("branch files = %#v from=%q err=%v", files, from, err)
	}
	diff, _, from, err = adapter.FileDiff(ctx, workspace, worktree.Path, worktree.BaseCommit, worktree.Branch, "a.md", 1<<20)
	if err != nil || from != "branch" || !strings.Contains(diff, "+b") {
		t.Fatalf("branch diff = %q from=%q err=%v", diff, from, err)
	}
	if _, _, _, err := adapter.FileDiff(ctx, workspace, worktree.Path, worktree.BaseCommit, worktree.Branch, "../etc/passwd", 1<<20); !errors.Is(err, ports.ErrNotChanged) {
		t.Fatalf("an unchanged path err = %v", err)
	}
}
