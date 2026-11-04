// Package gitworktree keeps each harness run in a git worktree under one root
// outside every workspace, so the workspace scanner never indexes it and the
// workspace's own checkout never moves.
package gitworktree

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"kode-stream/internal/common/models"
	appgit "kode-stream/internal/git"
	"kode-stream/internal/harness/ports"
)

// BranchPrefix is the only branch namespace the harness writes to.
const BranchPrefix = "harness/"

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
		return err
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

func (a *Adapter) Push(ctx context.Context, path, branch string) error {
	if err := a.inside(path); err != nil {
		return err
	}
	if err := checkBranch(branch, ""); err != nil {
		return err
	}
	return a.Git.PushBranchContext(ctx, path, a.Remote, branch)
}
