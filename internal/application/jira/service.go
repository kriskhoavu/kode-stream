package jira

import (
	"context"
	"errors"

	jiraclient "plan-manager/internal/jira"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
)

type Service struct {
	registry *registry.Registry
	client   *jiraclient.Client
}

func New(reg *registry.Registry, client *jiraclient.Client) *Service {
	return &Service{registry: reg, client: client}
}

func (s *Service) TestConnection(ctx context.Context, workspaceID string, connection *models.JiraConnection) (jiraclient.ConnectionTest, error) {
	if _, found, err := s.registry.Get(workspaceID); err != nil || !found {
		if err != nil {
			return jiraclient.ConnectionTest{}, err
		}
		return jiraclient.ConnectionTest{}, errors.New("workspace not found")
	}
	if connection == nil {
		return jiraclient.ConnectionTest{}, errors.New("Jira connection is required")
	}
	normalized, err := registry.ValidateJiraConnection(connection)
	if err != nil {
		return jiraclient.ConnectionTest{}, err
	}
	return s.client.TestConnection(ctx, *normalized)
}
