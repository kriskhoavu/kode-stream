package ports

import (
	"context"

	"kode-stream/internal/common/models"
)

type Worktree struct {
	Path       string
	Branch     string
	BaseCommit string
	// StartPoint is the ref the branch was cut from, such as origin/main.
	StartPoint string
}

type DiffFile struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// Worktrees gives every run a git worktree of its own, outside the
// workspace, on a harness/* branch. Only the harness commits and pushes.
type Worktrees interface {
	Create(ctx context.Context, workspace models.WorkspaceConfig, runID, branch string) (Worktree, error)
	Remove(ctx context.Context, workspace models.WorkspaceConfig, path, branch string) error
	Diff(ctx context.Context, path, base string) ([]DiffFile, string, error)
	RestorePaths(ctx context.Context, path, base string, paths []string) error
	Commit(ctx context.Context, path, message string) (string, error)
	Push(ctx context.Context, path, branch string) error
}
