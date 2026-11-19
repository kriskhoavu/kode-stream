// Package gitworktree keeps each harness run in a git worktree under one root
// outside every workspace, so the workspace scanner never indexes it and the
// workspace's own checkout never moves.
package gitworktree

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"kode-stream/internal/common/models"
	appgit "kode-stream/internal/git"
	"kode-stream/internal/harness/ports"
)

// BranchPrefix is the only local branch namespace the harness writes to.
const BranchPrefix = "harness/"

// RemoteBranchPrefix replaces BranchPrefix when a branch is pushed. Hosting
// hooks commonly accept only feature/<KEY>-<number>-..., which a harness
// branch, named <KEY>-<number>-<run>, then matches.
const RemoteBranchPrefix = "feature/"

func (a *Adapter) RemoteBranch(branch string) string {
	return RemoteBranchPrefix + strings.TrimPrefix(branch, BranchPrefix)
}

type Adapter struct {
	Root   string
	Git    *appgit.GitAdapter
	Remote string
}

func New(root string, git *appgit.GitAdapter) *Adapter {
	return &Adapter{Root: root, Git: git, Remote: "origin"}
}

// pathFor returns the run's worktree path and proves it stays under Root.
func (a *Adapter) pathFor(workspaceID, runID string) (string, error) {
	if a.Root == "" {
		return "", fmt.Errorf("the harness worktree root is not configured")
	}
	for _, segment := range []string{workspaceID, runID} {
		if segment == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, `/\`) {
			return "", fmt.Errorf("%q cannot name a worktree directory", segment)
		}
	}
	path := filepath.Join(a.Root, workspaceID, runID)
	return path, a.inside(path)
}

func (a *Adapter) inside(path string) error {
	root, err := filepath.Abs(a.Root)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("worktree path %s is outside %s", path, a.Root)
	}
	return nil
}

func checkBranch(branch, baseline string) error {
	if !strings.HasPrefix(branch, BranchPrefix) || len(branch) == len(BranchPrefix) {
		return fmt.Errorf("branch %s is not a harness/ branch", branch)
	}
	if branch == baseline {
		return fmt.Errorf("branch %s is the baseline", branch)
	}
	return nil
}

func (a *Adapter) Create(ctx context.Context, workspace models.WorkspaceConfig, runID, branch string) (ports.Worktree, error) {
	baseline := strings.TrimSpace(workspace.BaselineBranch)
	if baseline == "" {
		baseline = "main"
	}
	if err := checkBranch(branch, baseline); err != nil {
		return ports.Worktree{}, err
	}
	path, err := a.pathFor(workspace.ID, runID)
	if err != nil {
		return ports.Worktree{}, err
	}
	start := baseline
	if a.Remote != "" && a.Git.HasRemoteContext(ctx, workspace.Path, a.Remote) {
		if err := a.Git.FetchRemoteContext(ctx, workspace.Path, a.Remote); err != nil {
			return ports.Worktree{}, fmt.Errorf("fetch %s: %w", a.Remote, err)
		}
		if _, err := a.Git.RevParseContext(ctx, workspace.Path, a.Remote+"/"+baseline); err == nil {
			start = a.Remote + "/" + baseline
		}
	}
	base, err := a.Git.RevParseContext(ctx, workspace.Path, start)
	if err != nil {
		return ports.Worktree{}, err
	}
	if err := a.Git.ExcludeContext(ctx, workspace.Path, ".harness/"); err != nil {
		return ports.Worktree{}, err
	}
	if err := a.Git.WorktreeAddContext(ctx, workspace.Path, path, branch, base); err != nil {
		return ports.Worktree{}, err
	}
	return ports.Worktree{Path: path, Branch: branch, BaseCommit: base, StartPoint: start}, nil
}

func (a *Adapter) Remove(ctx context.Context, workspace models.WorkspaceConfig, path, branch string) error {
	if err := a.inside(path); err != nil {
		return err
	}
	if err := a.Git.WorktreeRemoveContext(ctx, workspace.Path, path, true); err != nil {
		_ = a.Git.WorktreePruneContext(ctx, workspace.Path)
		// A working copy the janitor already removed still leaves its branch to delete.
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			return err
		}
	}
	if branch != "" && strings.HasPrefix(branch, BranchPrefix) {
		_ = a.Git.DeleteBranchContext(ctx, workspace.Path, branch)
	}
	return a.Git.WorktreePruneContext(ctx, workspace.Path)
}

func (a *Adapter) Diff(ctx context.Context, path, base string) ([]ports.DiffFile, string, error) {
	if err := a.inside(path); err != nil {
		return nil, "", err
	}
	files, err := a.Git.DiffFilesContext(ctx, path, base)
	if err != nil {
		return nil, "", err
	}
	stat, err := a.Git.DiffStatContext(ctx, path, base)
	if err != nil {
		return nil, "", err
	}
	out := make([]ports.DiffFile, 0, len(files))
	for _, file := range files {
		out = append(out, ports.DiffFile{Path: file.Path, Status: file.Status, Additions: file.Additions, Deletions: file.Deletions})
	}
	return out, stat, nil
}

func (a *Adapter) RestorePaths(ctx context.Context, path, base string, paths []string) error {
	if err := a.inside(path); err != nil {
		return err
	}
	return a.Git.RestorePathsContext(ctx, path, base, paths)
}

func (a *Adapter) Commit(ctx context.Context, path, message string) (string, error) {
	if err := a.inside(path); err != nil {
		return "", err
	}
	return a.Git.CommitAllContext(ctx, path, message)
}

func (a *Adapter) Head(ctx context.Context, path string) (string, error) {
	if err := a.inside(path); err != nil {
		return "", err
	}
	return a.Git.RevParseContext(ctx, path, "HEAD")
}

func (a *Adapter) Push(ctx context.Context, path, branch string) error {
	if err := a.inside(path); err != nil {
		return err
	}
	if err := checkBranch(branch, ""); err != nil {
		return err
	}
	return a.Git.PushBranchContext(ctx, path, a.Remote, branch, a.RemoteBranch(branch))
}

// maxRunFiles bounds how many files one read returns.
const maxRunFiles = 40

func (a *Adapter) ReadRunFiles(ctx context.Context, workspace models.WorkspaceConfig, worktreePath, base, branch, prefix string, limit int64) ([]ports.RunFile, string, error) {
	if a.liveWorktree(worktreePath) {
		files, err := a.Git.DiffFilesContext(ctx, worktreePath, base)
		if err == nil {
			return readWorktreeFiles(worktreePath, files, prefix, limit), "worktree", nil
		}
	}
	ref, ok := a.branchRef(ctx, workspace, branch)
	if !ok {
		return []ports.RunFile{}, "", nil
	}
	files, err := a.Git.RefDiffFilesContext(ctx, workspace.Path, base, ref, prefix)
	if err != nil {
		return nil, "", err
	}
	out := []ports.RunFile{}
	for _, file := range files {
		if file.Status == "deleted" || len(out) == maxRunFiles {
			continue
		}
		data, truncated, err := a.Git.ShowFileContext(ctx, workspace.Path, ref, file.Path, limit)
		if err != nil {
			continue
		}
		if text, ok := textOf(data); ok {
			out = append(out, ports.RunFile{Path: file.Path, Status: file.Status, Content: text, Truncated: truncated})
		}
	}
	return out, "branch", nil
}

// liveWorktree reports whether a run's worktree is still on disk.
func (a *Adapter) liveWorktree(worktreePath string) bool {
	if worktreePath == "" || a.inside(worktreePath) != nil {
		return false
	}
	info, err := os.Stat(worktreePath)
	return err == nil && info.IsDir()
}

// branchRef is the run branch as the workspace holds it: the local harness/*
// branch, else the branch as last pushed.
func (a *Adapter) branchRef(ctx context.Context, workspace models.WorkspaceConfig, branch string) (string, bool) {
	if checkBranch(branch, "") != nil {
		return "", false
	}
	refs := []string{branch}
	if a.Remote != "" {
		refs = append(refs, a.Remote+"/"+a.RemoteBranch(branch))
	}
	for _, ref := range refs {
		if _, err := a.Git.RevParseContext(ctx, workspace.Path, ref); err == nil {
			return ref, true
		}
	}
	return "", false
}

func (a *Adapter) ChangedFiles(ctx context.Context, workspace models.WorkspaceConfig, worktreePath, base, branch string) ([]ports.DiffFile, string, error) {
	var files []appgit.DiffFile
	var err error
	from := "worktree"
	if a.liveWorktree(worktreePath) {
		files, err = a.Git.DiffFilesContext(ctx, worktreePath, base)
	} else if ref, ok := a.branchRef(ctx, workspace, branch); ok {
		from = "branch"
		files, err = a.Git.RefDiffFilesContext(ctx, workspace.Path, base, ref, "")
	} else {
		return []ports.DiffFile{}, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	out := make([]ports.DiffFile, 0, len(files))
	for _, file := range files {
		out = append(out, ports.DiffFile{Path: file.Path, Status: file.Status, Additions: file.Additions, Deletions: file.Deletions})
	}
	return out, from, nil
}

func (a *Adapter) FileDiff(ctx context.Context, workspace models.WorkspaceConfig, worktreePath, base, branch, path string, limit int64) (string, bool, string, error) {
	files, from, err := a.ChangedFiles(ctx, workspace, worktreePath, base, branch)
	if err != nil || from == "" {
		return "", false, from, err
	}
	changed := false
	for _, file := range files {
		changed = changed || file.Path == path
	}
	if !changed {
		return "", false, from, ports.ErrNotChanged
	}
	if from == "worktree" {
		diff, truncated, err := a.Git.DiffFileSince(ctx, worktreePath, base, path, limit)
		return diff, truncated, from, err
	}
	ref, _ := a.branchRef(ctx, workspace, branch)
	diff, truncated, err := a.Git.RefFileDiffContext(ctx, workspace.Path, base, ref, path, limit)
	return diff, truncated, from, err
}

// readWorktreeFiles reads the changed files under prefix from a worktree,
// refusing a symbolic link and any path that leaves the worktree.
func readWorktreeFiles(root string, files []appgit.DiffFile, prefix string, limit int64) []ports.RunFile {
	out := []ports.RunFile{}
	for _, file := range files {
		if file.Status == "deleted" || !strings.HasPrefix(file.Path, prefix) || len(out) == maxRunFiles {
			continue
		}
		clean := filepath.Clean(filepath.FromSlash(file.Path))
		if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			continue
		}
		full := filepath.Join(root, clean)
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if info.Size() > limit {
			out = append(out, ports.RunFile{Path: file.Path, Status: file.Status, Truncated: true})
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		if text, ok := textOf(data); ok {
			out = append(out, ports.RunFile{Path: file.Path, Status: file.Status, Content: text})
		}
	}
	return out
}

// textOf returns data as text, or false for a binary file.
func textOf(data []byte) (string, bool) {
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return "", false
	}
	return string(data), true
}
