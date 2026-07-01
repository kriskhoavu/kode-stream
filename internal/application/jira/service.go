package jira

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"plan-manager/internal/itemindex"
	jiraclient "plan-manager/internal/jira"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
)

type IssueState struct {
	State        string            `json:"state"`
	Issue        *jiraclient.Issue `json:"issue,omitempty"`
	Message      string            `json:"message,omitempty"`
	RecoveryHint string            `json:"recoveryHint,omitempty"`
	RefreshedAt  time.Time         `json:"refreshedAt,omitempty"`
}

type cacheEntry struct {
	state     IssueState
	expiresAt time.Time
}

type Service struct {
	registry *registry.Registry
	index    *itemindex.Index
	client   *jiraclient.Client
	mu       sync.Mutex
	cache    map[string]cacheEntry
	now      func() time.Time
}

func New(reg *registry.Registry, index *itemindex.Index, client *jiraclient.Client) *Service {
	return &Service{registry: reg, index: index, client: client, cache: map[string]cacheEntry{}, now: time.Now}
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

func (s *Service) Issue(ctx context.Context, itemID string, refresh bool) (IssueState, error) {
	item, found, err := s.index.Get(itemID)
	if err != nil {
		return IssueState{}, err
	}
	if !found {
		return IssueState{}, errors.New("item not found")
	}
	workspace, found, err := s.registry.Get(item.WorkspaceID)
	if err != nil {
		return IssueState{}, err
	}
	if !found {
		return IssueState{}, errors.New("workspace not found")
	}
	if workspace.Jira == nil {
		return IssueState{State: "not_configured", Message: "Jira is not configured for this workspace"}, nil
	}
	key := strings.ToUpper(strings.TrimSpace(item.Identifier))
	project := workspace.Jira.ProjectKey
	if !regexp.MustCompile(`^[A-Z][A-Z0-9_]*-[1-9][0-9]*$`).MatchString(key) {
		return IssueState{State: "invalid_identifier", Message: "Item identifier is not a Jira issue key"}, nil
	}
	if !strings.HasPrefix(key, project+"-") {
		return IssueState{State: "project_mismatch", Message: fmt.Sprintf("Item belongs to a different Jira project than %s", project)}, nil
	}
	cacheKey := workspace.ID + "|" + workspace.Jira.BaseURL + "|" + key
	if !refresh {
		s.mu.Lock()
		entry, ok := s.cache[cacheKey]
		s.mu.Unlock()
		if ok && s.now().Before(entry.expiresAt) {
			return entry.state, nil
		}
	}
	issue, err := s.client.GetIssue(ctx, *workspace.Jira, key)
	state := IssueState{RefreshedAt: s.now().UTC()}
	switch {
	case err == nil:
		state.State = "available"
		state.Issue = &issue
	case errors.Is(err, jiraclient.ErrNotFound):
		state.State = "not_found"
		state.Message = "No Jira ticket exists for this item"
	case errors.Is(err, jiraclient.ErrAuthentication):
		state.State = "authentication_failed"
		state.Message = err.Error()
		state.RecoveryHint = "Check the configured token environment variable and restart Plan Manager."
	case errors.Is(err, jiraclient.ErrForbidden):
		state.State = "forbidden"
		state.Message = err.Error()
		state.RecoveryHint = "Request permission to view this Jira project."
	default:
		state.State = "unavailable"
		state.Message = err.Error()
		state.RecoveryHint = "Check Jira availability and the workspace connection settings."
	}
	if state.State == "available" || state.State == "not_found" {
		s.mu.Lock()
		s.cache[cacheKey] = cacheEntry{state: state, expiresAt: s.now().Add(5 * time.Minute)}
		s.mu.Unlock()
	}
	return state, nil
}
