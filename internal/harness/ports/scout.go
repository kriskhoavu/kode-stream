package ports

import (
	"context"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
)

type Scout interface {
	Kind() domain.SourceKind
	Enabled(workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) bool
	Poll(ctx context.Context, workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) ([]domain.Candidate, error)
}
