package jira

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/item/index"
	"kode-stream/internal/workspace/registry"
)

type IssueState struct {
	State        string    `json:"state"`
	Issue        *Issue    `json:"issue,omitempty"`
	Message      string    `json:"message,omitempty"`
	RecoveryHint string    `json:"recoveryHint,omitempty"`
	RefreshedAt  time.Time `json:"refreshedAt,omitempty"`
}

type cacheEntry struct {
	state     IssueState
	expiresAt time.Time
}

type JiraService struct {
	registry registry.Repository
	index    itemindex.Repository
	client   *Client
	mu       sync.Mutex
	cache    map[string]cacheEntry
	now      func() time.Time
}

type Service = JiraService

func NewService(reg registry.Repository, index itemindex.Repository, client *Client) *JiraService {
	return &JiraService{registry: reg, index: index, client: client, cache: map[string]cacheEntry{}, now: time.Now}
}

func (s *Service) TestConnection(ctx context.Context, workspaceID string, connection *models.JiraConnection) (ConnectionTest, error) {
	if _, found, err := s.registry.Get(workspaceID); err != nil || !found {
		if err != nil {
			return ConnectionTest{}, err
		}
		return ConnectionTest{}, errors.New("workspace not found")
	}
	if connection == nil {
		return ConnectionTest{}, errors.New("Jira connection is required")
	}
	normalized, err := registry.ValidateJiraConnection(connection)
	if err != nil {
		return ConnectionTest{}, err
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
	return s.issueByKey(ctx, workspace, item.Identifier, refresh)
}

func (s *Service) WorkspaceIssue(ctx context.Context, workspaceID, issueKey string, refresh bool) (IssueState, error) {
	workspace, found, err := s.registry.Get(workspaceID)
	if err != nil {
		return IssueState{}, err
	}
	if !found {
		return IssueState{}, errors.New("workspace not found")
	}
	return s.issueByKey(ctx, workspace, issueKey, refresh)
}

func (s *Service) issueByKey(ctx context.Context, workspace models.WorkspaceConfig, issueKey string, refresh bool) (IssueState, error) {
	if workspace.Jira == nil {
		return IssueState{State: "not_configured", Message: "Jira is not configured for this workspace"}, nil
	}
	key := strings.ToUpper(strings.TrimSpace(issueKey))
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
	case errors.Is(err, ErrNotFound):
		state.State = "not_found"
		state.Message = "No Jira ticket exists for this item"
	case errors.Is(err, ErrAuthentication):
		state.State = "authentication_failed"
		state.Message = err.Error()
		state.RecoveryHint = "Check the configured token environment variable and restart Kode Stream."
	case errors.Is(err, ErrForbidden):
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

func (s *Service) Attachment(ctx context.Context, itemID, attachmentID string) (AttachmentContent, error) {
	state, err := s.Issue(ctx, itemID, false)
	if err != nil {
		return AttachmentContent{}, err
	}
	if state.State != "available" || state.Issue == nil {
		return AttachmentContent{}, errors.New("Jira issue is unavailable")
	}
	var attachment *Attachment
	for index := range state.Issue.Attachments {
		if state.Issue.Attachments[index].ID == attachmentID {
			attachment = &state.Issue.Attachments[index]
			break
		}
	}
	if attachment == nil {
		return AttachmentContent{}, errors.New("Jira attachment does not belong to this issue")
	}
	item, found, err := s.index.Get(itemID)
	if err != nil || !found {
		if err != nil {
			return AttachmentContent{}, err
		}
		return AttachmentContent{}, errors.New("item not found")
	}
	workspace, found, err := s.registry.Get(item.WorkspaceID)
	if err != nil || !found || workspace.Jira == nil {
		if err != nil {
			return AttachmentContent{}, err
		}
		return AttachmentContent{}, errors.New("workspace not found")
	}
	return s.client.GetAttachment(ctx, *workspace.Jira, *attachment)
}
