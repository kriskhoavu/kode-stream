package ports

import (
	"context"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
)

type PullRequest struct {
	Workspace    models.WorkspaceConfig
	Settings     domain.WorkspaceSettings
	WorktreePath string
	Branch       string
	Base         string
	Title        string
	Body         string
}

// PullRequests opens a draft pull request for a pushed harness branch. It
// never merges, labels or comments.
type PullRequests interface {
	ID() string
	Supports(workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) bool
	CreateDraft(ctx context.Context, request PullRequest) (string, error)
}

// Audit records what the harness did outside its own tables.
type Audit interface {
	Append(models.AuditEvent) (models.AuditEvent, error)
}
