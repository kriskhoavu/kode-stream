package ports

import (
	"context"
	"errors"

	"kode-stream/internal/common/models"
)

type Worktree struct {
	Path       string
	Branch     string
	BaseCommit string
	StartPoint string
}

type DiffFile struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type RunFile struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

type Worktrees interface {
	Create(ctx context.Context, workspace models.WorkspaceConfig, runID, branch string) (Worktree, error)
	Remove(ctx context.Context, workspace models.WorkspaceConfig, path, branch string) error
	Diff(ctx context.Context, path, base string) ([]DiffFile, string, error)
	RestorePaths(ctx context.Context, path, base string, paths []string) error
	Commit(ctx context.Context, path, message string) (string, error)
	Head(ctx context.Context, path string) (string, error)
	Push(ctx context.Context, path, branch string) error
	RemoteBranch(branch string) string
	// ReadRunFiles and ChangedFiles say where they read: "worktree", "branch", or "" when neither is left.
	ReadRunFiles(ctx context.Context, workspace models.WorkspaceConfig, worktreePath, base, branch, prefix string, limit int64) ([]RunFile, string, error)
	ChangedFiles(ctx context.Context, workspace models.WorkspaceConfig, worktreePath, base, branch string) ([]DiffFile, string, error)
	// A path the run did not change is ErrNotChanged.
	FileDiff(ctx context.Context, workspace models.WorkspaceConfig, worktreePath, base, branch, path string, limit int64) (string, bool, string, error)
}

var ErrNotChanged = errors.New("the run did not change this path")
