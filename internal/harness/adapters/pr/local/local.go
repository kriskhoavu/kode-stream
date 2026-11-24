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

func (a *Adapter) Supports(_ models.WorkspaceConfig, settings domain.WorkspaceSettings) bool {
	return settings.Local != nil && settings.Local.Enabled
}

func (a *Adapter) CreateDraft(context.Context, ports.PullRequest) (string, error) {
	return "", nil
}
