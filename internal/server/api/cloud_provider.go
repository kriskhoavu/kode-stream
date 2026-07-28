package api

import (
	"context"
	"fmt"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/provider"
)

// cloudProviderStore keeps administrator-owned provider endpoints separate
// from opaque, user-owned connection material.
type cloudProviderStore struct {
	registry    *provider.Registry
	connections *provider.ConnectionStore
}

func newCloudProviderStore() *cloudProviderStore {
	return &cloudProviderStore{
		registry:    provider.NewRegistry(provider.GitHubFactory{}, provider.BitbucketServerFactory{}),
		connections: provider.NewConnectionStore(),
	}
}

func (s *cloudProviderStore) integration(ctx context.Context, userID string, workspace models.WorkspaceConfig) (provider.GitProviderIntegration, error) {
	if strings.TrimSpace(workspace.ProviderInstanceID) == "" || strings.TrimSpace(workspace.ProviderRepository) == "" {
		return nil, fmt.Errorf("provider instance and repository are required")
	}
	token, ok := s.connections.Token(userID, workspace.ProviderInstanceID)
	if !ok {
		return nil, fmt.Errorf("provider connection is required")
	}
	integration, err := s.registry.Integration(workspace.ProviderInstanceID, token)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(workspace.Provider) != "" {
		if _, err := provider.Require(integration, workspace.Provider); err != nil {
			return nil, err
		}
	}
	repositories, err := integration.Repositories(ctx)
	if err != nil {
		return nil, err
	}
	for _, repository := range repositories {
		if repository.FullName == workspace.ProviderRepository || repository.ID == workspace.ProviderRepository {
			return integration, nil
		}
	}
	return nil, fmt.Errorf("provider repository is not authorized")
}

type remoteSnapshotAdapter struct{ providers *cloudProviderStore }

func (a remoteSnapshotAdapter) Command(_ cloudSession, _ models.WorkspaceConfig, _ cloudCommandInput) (models.CommandResult, int, string) {
	return models.CommandResult{}, 0, "remote snapshot workspaces support read-only snapshot access; Cloud cannot run file, Git, terminal, AI, runtime, or verification commands"
}

func (a remoteSnapshotAdapter) Resolve(ctx context.Context, session cloudSession, workspace models.WorkspaceConfig) (models.WorkspaceConfig, provider.GitProviderIntegration, error) {
	integration, err := a.providers.integration(ctx, session.User.ID, workspace)
	if err != nil {
		return models.WorkspaceConfig{}, nil, err
	}
	ref, err := integration.ResolveRef(ctx, workspace.ProviderRepository, workspace.SelectedRef)
	if err != nil {
		return models.WorkspaceConfig{}, nil, err
	}
	workspace.ResolvedCommitSHA = ref.CommitSHA
	return workspace, integration, nil
}
