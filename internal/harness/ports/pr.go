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

type PullRequests interface {
	ID() string
	Supports(workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) bool
	CreateDraft(ctx context.Context, request PullRequest) (string, error)
}

type Audit interface {
	Append(models.AuditEvent) (models.AuditEvent, error)
}
