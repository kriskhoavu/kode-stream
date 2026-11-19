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
	// StartPoint is the ref the branch was cut from, such as origin/main.
	StartPoint string
}

type DiffFile struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// RunFile is one file a run wrote, read for a person to look at. Content is
// empty when Truncated: the file is larger than the reader's limit.
type RunFile struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Worktrees gives every run a git worktree of its own, outside the
// workspace, on a harness/* branch. Only the harness commits and pushes.
type Worktrees interface {
	Create(ctx context.Context, workspace models.WorkspaceConfig, runID, branch string) (Worktree, error)
	Remove(ctx context.Context, workspace models.WorkspaceConfig, path, branch string) error
	Diff(ctx context.Context, path, base string) ([]DiffFile, string, error)
	RestorePaths(ctx context.Context, path, base string, paths []string) error
	Commit(ctx context.Context, path, message string) (string, error)
	// Head is the commit the worktree at path is at.
	Head(ctx context.Context, path string) (string, error)
	// Push pushes the local harness/* branch under RemoteBranch's name.
	Push(ctx context.Context, path, branch string) error
	// RemoteBranch is the name a local harness/* branch has at the remote.
	RemoteBranch(branch string) string
	// ReadRunFiles reads the files under prefix a run changed against base:
	// from its worktree while it exists, uncommitted work included, else from
	// its branch, local or as last pushed. It names where it read from,
	// "worktree" or "branch", or "" when neither is left.
	ReadRunFiles(ctx context.Context, workspace models.WorkspaceConfig, worktreePath, base, branch, prefix string, limit int64) ([]RunFile, string, error)
	// ChangedFiles lists what a run changed against base, from its worktree
	// while it exists, else from its branch, saying which: "worktree",
	// "branch", or "" when neither is left.
	ChangedFiles(ctx context.Context, workspace models.WorkspaceConfig, worktreePath, base, branch string) ([]DiffFile, string, error)
	// FileDiff returns the diff of one changed path, read the same way and
	// bounded by limit. A path the run did not change is ErrNotChanged.
	FileDiff(ctx context.Context, workspace models.WorkspaceConfig, worktreePath, base, branch, path string, limit int64) (string, bool, string, error)
}

// ErrNotChanged is a diff asked for a path the run did not change.
var ErrNotChanged = errors.New("the run did not change this path")
