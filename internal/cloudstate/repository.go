package cloudstate

import (
	"context"

	"kode-stream/internal/common/models"
	"kode-stream/internal/provider"
)

// Repository is the durable, tenant-scoped portion of the Cloud control plane.
// WebSocket ownership, pending commands, and process bindings intentionally do
// not belong here.
type Repository interface {
	ListWorkspaces(context.Context, string) ([]models.WorkspaceConfig, error)
	GetWorkspace(context.Context, string, string) (models.WorkspaceConfig, bool, error)
	UpsertWorkspace(context.Context, string, models.WorkspaceConfig) (models.WorkspaceConfig, error)
	ListAgents(context.Context, string) ([]models.CloudAgent, error)
	UpsertAgent(context.Context, string, models.CloudAgent) (models.CloudAgent, error)
	GetProviderInstance(context.Context, string, string) (provider.Instance, bool, error)
	UpsertProviderInstance(context.Context, string, provider.Instance) error
	GetEncryptedConnection(context.Context, string, string) (string, bool, error)
	SaveEncryptedConnection(context.Context, string, string, string) error
	RevokeConnection(context.Context, string, string) error
}
