package cloudstate

import (
	"context"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/provider"
)

type EnrollmentTokenRepository interface {
	ConsumeEnrollmentToken(context.Context, string, time.Time) (bool, error)
}

// WorkspacePublicationRepository gives Agent publication an explicit durable
// compare-and-swap boundary.  It is separate from ordinary Cloud workspace
// upserts because browser-created snapshots do not carry Agent generations.
type WorkspacePublicationRepository interface {
	PublishAgentWorkspace(context.Context, string, models.WorkspaceConfig, int64) (models.WorkspaceConfig, bool, error)
}

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
