// Package local hands off a run of a local task: the harness has already
// pushed its branch to the checkout's origin, and there is no pull request
// to open. It never merges.
package local

import (
	"context"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

func (a *Adapter) ID() string { return string(domain.SourceLocal) }

// Supports a workspace whose local task folder is on: its runs hand off
// without a pull request, whatever its origin is.
func (a *Adapter) Supports(_ models.WorkspaceConfig, settings domain.WorkspaceSettings) bool {
	return settings.Local != nil && settings.Local.Enabled
}

// CreateDraft opens nothing and returns no address.
func (a *Adapter) CreateDraft(context.Context, ports.PullRequest) (string, error) {
	return "", nil
}
