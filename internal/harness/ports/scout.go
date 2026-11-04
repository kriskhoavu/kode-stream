package ports

import (
	"context"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
)

// Scout reads one kind of issue source for one workspace. It only reads.
type Scout interface {
	Kind() domain.SourceKind
	// Enabled reports whether the settings configure this source for the
	// workspace.
	Enabled(workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) bool
	// Poll returns every open issue the source lists now. A task the source
	// stops listing is marked stale after three polls.
	Poll(ctx context.Context, workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) ([]domain.Candidate, error)
}
