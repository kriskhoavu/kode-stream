package api

import (
	"context"
	"fmt"
	"strings"

	"kode-stream/internal/cloudstate"
	"kode-stream/internal/common/models"
	"kode-stream/internal/provider"
)

// cloudProviderStore keeps administrator-owned provider endpoints separate
// from opaque, user-owned connection material.
type cloudProviderStore struct {
	registry    *cloudProviderRegistry
	connections *cloudConnectionStore
}

const cloudProviderSystemOwner = "system"

type cloudProviderRegistry struct {
	memory      *provider.Registry
	persistence cloudstate.Repository
}

func (r *cloudProviderRegistry) Upsert(instance provider.Instance) error {
	if err := provider.ValidateInstance(instance); err != nil {
		return err
	}
	if instance.Kind != "github" && instance.Kind != "bitbucket_server" {
		return fmt.Errorf("unsupported provider kind %q", instance.Kind)
	}
	if r.persistence != nil {
		if err := r.persistence.UpsertProviderInstance(context.Background(), cloudProviderSystemOwner, instance); err != nil {
			return err
		}
	}
	return r.memory.Upsert(instance)
}

func (r *cloudProviderRegistry) Integration(ctx context.Context, instanceID, token string) (provider.GitProviderIntegration, error) {
	integration, err := r.memory.Integration(instanceID, token)
	if err == nil || r.persistence == nil {
		return integration, err
	}
	instance, found, loadErr := r.persistence.GetProviderInstance(ctx, cloudProviderSystemOwner, instanceID)
	if loadErr != nil {
		return nil, loadErr
	}
	if !found {
		return nil, err
	}
	if err := r.memory.Upsert(instance); err != nil {
		return nil, err
	}
	return r.memory.Integration(instanceID, token)
}

type cloudConnectionStore struct {
	memory      *provider.ConnectionStore
	persistence cloudstate.Repository
	cipher      *provider.CredentialCipher
}

func (s *cloudConnectionStore) Save(userID, instanceID, token string) error {
	if s.persistence == nil {
		return s.memory.Save(userID, instanceID, token)
	}
	encrypted, err := s.cipher.Encrypt(token)
	if err != nil {
		return err
	}
	return s.persistence.SaveEncryptedConnection(context.Background(), userID, instanceID, encrypted)
}

func (s *cloudConnectionStore) Token(ctx context.Context, userID, instanceID string) (string, bool, error) {
	if s.persistence == nil {
		token, ok := s.memory.Token(userID, instanceID)
		return token, ok, nil
	}
	encrypted, found, err := s.persistence.GetEncryptedConnection(ctx, userID, instanceID)
	if err != nil || !found {
		return "", found, err
	}
	token, err := s.cipher.Decrypt(encrypted)
	return token, err == nil, err
}

func (s *cloudConnectionStore) Revoke(ctx context.Context, userID, instanceID string) error {
	if s.persistence == nil {
		s.memory.Revoke(userID, instanceID)
		return nil
	}
	return s.persistence.RevokeConnection(ctx, userID, instanceID)
}

func newCloudProviderStore(persistence cloudstate.Repository, secret string) *cloudProviderStore {
	return &cloudProviderStore{
		registry:    &cloudProviderRegistry{memory: provider.NewRegistry(provider.GitHubFactory{}, provider.BitbucketServerFactory{}), persistence: persistence},
		connections: &cloudConnectionStore{memory: provider.NewConnectionStore(), persistence: persistence, cipher: provider.NewCredentialCipher(secret)},
	}
}

func (s *cloudProviderStore) integration(ctx context.Context, userID string, workspace models.WorkspaceConfig) (provider.GitProviderIntegration, error) {
	if strings.TrimSpace(workspace.ProviderInstanceID) == "" || strings.TrimSpace(workspace.ProviderRepository) == "" {
		return nil, fmt.Errorf("provider instance and repository are required")
	}
	token, ok, err := s.connections.Token(ctx, userID, workspace.ProviderInstanceID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("provider connection is required")
	}
	integration, err := s.registry.Integration(ctx, workspace.ProviderInstanceID, token)
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
