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

type inFlightIssue struct {
	done       chan struct{}
	cancel     context.CancelFunc
	state      IssueState
	waiters    int
	abandoned  bool
	generation uint64
}

type JiraService struct {
	registry   registry.Repository
	index      itemindex.Repository
	client     *Client
	mu         sync.Mutex
	cache      map[string]cacheEntry
	inFlight   map[string]*inFlightIssue
	generation map[string]uint64
	now        func() time.Time
}

type Service = JiraService

func NewService(reg registry.Repository, index itemindex.Repository, client *Client) *JiraService {
	return &JiraService{registry: reg, index: index, client: client, cache: map[string]cacheEntry{}, inFlight: map[string]*inFlightIssue{}, generation: map[string]uint64{}, now: time.Now}
}

// InvalidateWorkspace drops only process-local values after a workspace change
// was committed. It is deliberately a narrow lifecycle seam, not a registry wrapper.
func (s *Service) InvalidateWorkspace(workspaceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.cache {
		if strings.HasPrefix(key, workspaceID+"|") {
			delete(s.cache, key)
		}
	}
	for key := range s.generation {
		if strings.HasPrefix(key, workspaceID+"|") {
			s.generation[key]++
		}
	}
}

const maxCacheEntries = 256

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
	cacheKey := jiraCacheKey(workspace.ID, *workspace.Jira, key)
	if !refresh {
		s.mu.Lock()
		s.pruneCacheLocked(s.now())
		entry, ok := s.cache[cacheKey]
		if ok && s.now().Before(entry.expiresAt) {
			s.mu.Unlock()
			return entry.state, nil
		}
		if flight := s.inFlight[cacheKey]; flight != nil {
			flight.waiters++
			s.mu.Unlock()
			return s.waitForFlight(ctx, cacheKey, flight)
		}
		s.generation[cacheKey]++
		generation := s.generation[cacheKey]
		fetchCtx, cancel := context.WithCancel(context.Background())
		flight := &inFlightIssue{done: make(chan struct{}), cancel: cancel, waiters: 1, generation: generation}
		s.inFlight[cacheKey] = flight
		s.mu.Unlock()
		go s.completeFlight(fetchCtx, cacheKey, flight, *workspace.Jira, key)
		return s.waitForFlight(ctx, cacheKey, flight)
	}
	s.mu.Lock()
	s.generation[cacheKey]++
	generation := s.generation[cacheKey]
	s.mu.Unlock()
	state := s.fetchIssue(ctx, *workspace.Jira, key)
	if state.State == "available" || state.State == "not_found" {
		s.mu.Lock()
		if s.generation[cacheKey] == generation {
			s.putCacheLocked(cacheKey, state)
		}
		s.mu.Unlock()
	}
	return state, nil
}

func (s *Service) waitForFlight(ctx context.Context, cacheKey string, flight *inFlightIssue) (IssueState, error) {
	select {
	case <-flight.done:
		return flight.state, nil
	case <-ctx.Done():
		s.mu.Lock()
		flight.waiters--
		if flight.waiters == 0 && !flight.abandoned {
			flight.abandoned = true
			flight.cancel()
		}
		s.mu.Unlock()
		return IssueState{}, ctx.Err()
	}
}

func (s *Service) completeFlight(ctx context.Context, cacheKey string, flight *inFlightIssue, connection models.JiraConnection, key string) {
	state := s.fetchIssue(ctx, connection, key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inFlight[cacheKey] == flight {
		delete(s.inFlight, cacheKey)
	}
	flight.state = state
	if !flight.abandoned && s.generation[cacheKey] == flight.generation && (state.State == "available" || state.State == "not_found") {
		s.putCacheLocked(cacheKey, state)
	}
	close(flight.done)
}

func jiraCacheKey(workspaceID string, c models.JiraConnection, issueKey string) string {
	return strings.Join([]string{workspaceID, strings.ToLower(strings.TrimSpace(c.DeploymentType)), strings.ToLower(strings.TrimSpace(c.BaseURL)), strings.ToUpper(strings.TrimSpace(c.ProjectKey)), strings.TrimSpace(c.AccountEmail), strings.TrimSpace(c.TokenEnvVar), issueKey}, "|")
}

func (s *Service) fetchIssue(ctx context.Context, connection models.JiraConnection, key string) IssueState {
	issue, err := s.client.GetIssue(ctx, connection, key)
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
	return state
}

func (s *Service) pruneCacheLocked(now time.Time) {
	for key, entry := range s.cache {
		if !now.Before(entry.expiresAt) {
			delete(s.cache, key)
		}
	}
}
func (s *Service) putCacheLocked(key string, state IssueState) {
	s.pruneCacheLocked(s.now())
	if len(s.cache) >= maxCacheEntries {
		var oldest string
		var until time.Time
		for candidate, entry := range s.cache {
			if oldest == "" || entry.expiresAt.Before(until) {
				oldest, until = candidate, entry.expiresAt
			}
		}
		delete(s.cache, oldest)
	}
	s.cache[key] = cacheEntry{state: state, expiresAt: s.now().Add(5 * time.Minute)}
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
