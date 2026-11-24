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

// Kept apart from workspace upserts: browser-created snapshots carry no Agent generation.
type WorkspacePublicationRepository interface {
	PublishAgentWorkspace(context.Context, string, models.WorkspaceConfig, int64) (models.WorkspaceConfig, bool, error)
}

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
