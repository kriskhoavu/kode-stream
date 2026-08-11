package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"kode-stream/internal/cloudstate"
	"kode-stream/internal/common/models"
)

type cloudWorkspaceStore struct {
	mu          sync.RWMutex
	workspaces  map[string]map[string]models.WorkspaceConfig
	persistence cloudstate.Repository
}

func newCloudWorkspaceStore(persistence ...cloudstate.Repository) *cloudWorkspaceStore {
	var repository cloudstate.Repository
	if len(persistence) > 0 {
		repository = persistence[0]
	}
	return &cloudWorkspaceStore{workspaces: map[string]map[string]models.WorkspaceConfig{}, persistence: repository}
}

func (s *cloudWorkspaceStore) List(ctx context.Context, userID string) ([]models.WorkspaceConfig, error) {
	if s.persistence != nil {
		return s.persistence.ListWorkspaces(ctx, userID)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	owned := s.workspaces[userID]
	if len(owned) == 0 {
		return []models.WorkspaceConfig{}, nil
	}
	result := make([]models.WorkspaceConfig, 0, len(owned))
	for _, workspace := range owned {
		result = append(result, workspace)
	}
	return result, nil
}

func (s *cloudWorkspaceStore) Get(ctx context.Context, userID, workspaceID string) (models.WorkspaceConfig, bool, error) {
	if s.persistence != nil {
		return s.persistence.GetWorkspace(ctx, userID, workspaceID)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspace, ok := s.workspaces[userID][workspaceID]
	return workspace, ok, nil
}

func (s *cloudWorkspaceStore) Upsert(ctx context.Context, workspace models.WorkspaceConfig) (models.WorkspaceConfig, error) {
	workspace = normalizeCloudWorkspaceAccess(workspace)
	if s.persistence != nil {
		return s.persistence.UpsertWorkspace(ctx, workspace.OwnerUserID, workspace)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workspaces[workspace.OwnerUserID] == nil {
		s.workspaces[workspace.OwnerUserID] = map[string]models.WorkspaceConfig{}
	}
	s.workspaces[workspace.OwnerUserID][workspace.ID] = workspace
	return workspace, nil
}

func normalizeCloudWorkspaceAccess(workspace models.WorkspaceConfig) models.WorkspaceConfig {
	if workspace.AccessMode == "" {
		workspace.AccessMode = models.WorkspaceAccessModeAgentBacked
	}
	if workspace.AccessMode == models.WorkspaceAccessModeAgentBacked {
		workspace.Location = models.WorkspaceLocationCloudAgent
	}
	if workspace.AccessMode == models.WorkspaceAccessModeRemoteSnapshot {
		workspace.Location = models.WorkspaceLocationCloudRemoteSnapshot
		workspace.Path = ""
		workspace.AgentID = ""
		workspace.LocalRootLabel = ""
		workspace.RemoteURL = ""
	}
	return workspace
}

func (a *cloudController) registerCloudWorkspaceFromAgent(w http.ResponseWriter, r *http.Request) {
	token, ok := a.verifyAgentToken(r.Header.Get("Authorization"))
	if !ok {
		token, ok = a.verifyAgentToken(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid agent credential")
		return
	}
	var input struct {
		Name             string   `json:"name"`
		BaselineBranch   string   `json:"baselineBranch"`
		Sources          []string `json:"sources"`
		RemoteURL        string   `json:"remoteUrl"`
		LocalRootLabel   string   `json:"localRootLabel"`
		PublishedSummary bool     `json:"publishedSummary"`
		ScanStatus       string   `json:"scanStatus"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "workspace name is required")
		return
	}
	branch := strings.TrimSpace(input.BaselineBranch)
	if branch == "" {
		branch = "main"
	}
	sources := normalizeCloudSources(input.Sources)
	workspace := models.WorkspaceConfig{
		ID:               stableCloudUserID(token.UserID + ":" + token.AgentID + ":" + name),
		Name:             name,
		Path:             "",
		Location:         models.WorkspaceLocationCloudAgent,
		AccessMode:       models.WorkspaceAccessModeAgentBacked,
		OwnerUserID:      token.UserID,
		AgentID:          token.AgentID,
		LocalRootLabel:   redactRootLabel(input.LocalRootLabel),
		RemoteURL:        strings.TrimSpace(input.RemoteURL),
		PublishedSummary: input.PublishedSummary,
		ScanStatus:       strings.TrimSpace(input.ScanStatus),
		BaselineBranch:   branch,
		RegistrationMode: models.WorkspaceRegistrationModeExisting,
		Sources:          sources,
		CreatedAt:        time.Now().UTC(),
		LastScannedAt:    time.Now().UTC(),
	}
	if workspace.ScanStatus == "" {
		workspace.ScanStatus = "published"
	}
	created, err := a.workspaces.Upsert(r.Context(), workspace)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Cloud workspace persistence is unavailable")
		return
	}
	if a.audit != nil {
		_, _ = a.audit.Append(models.AuditEvent{OwnerUserID: token.UserID, ActorUserID: token.UserID, WorkspaceID: created.ID, Operation: "cloud_workspace_register", Status: models.AuditStatusSuccess, Message: "Cloud Agent workspace registered", Time: time.Now().UTC(), Paths: []string{}})
	}
	writeJSON(w, http.StatusCreated, created)
}

func normalizeCloudSources(sources []string) []string {
	if len(sources) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(sources))
	for _, source := range sources {
		clean := strings.Trim(strings.TrimSpace(source), "/")
		if clean != "" && !strings.Contains(clean, "..") {
			result = append(result, clean)
		}
	}
	return result
}

func redactRootLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	parts := strings.FieldsFunc(label, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return label
	}
	return ".../" + parts[len(parts)-1]
}

func (a *workspaceController) rejectCloudBrowserWorkspaceRegistration(w http.ResponseWriter, input models.WorkspaceInput) bool {
	if a.runtimeConfig.Mode != models.RuntimeModeCloud {
		return false
	}
	if strings.TrimSpace(input.Path) != "" {
		writeError(w, http.StatusBadRequest, "Cloud workspaces must be registered by Cloud Agent")
		return true
	}
	if input.RegistrationMode == models.WorkspaceRegistrationModeRemoteClone || strings.TrimSpace(input.RemoteURL) != "" {
		writeError(w, http.StatusBadRequest, "Cloud does not clone repositories")
		return true
	}
	writeError(w, http.StatusBadRequest, "Cloud workspaces must be registered by Cloud Agent")
	return true
}

func (a *workspaceController) createCloudRemoteSnapshotWorkspace(w http.ResponseWriter, r *http.Request, input models.WorkspaceInput) {
	session, ok := cloudSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Cloud session is required")
		return
	}
	if input.AccessMode != models.WorkspaceAccessModeRemoteSnapshot {
		writeError(w, http.StatusBadRequest, "Cloud browser registration supports Remote Snapshot workspaces; use Cloud Agent registration for agent-backed workspaces")
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || strings.TrimSpace(input.ProviderInstanceID) == "" || strings.TrimSpace(input.ProviderRepository) == "" || strings.TrimSpace(input.SelectedRef) == "" {
		writeError(w, http.StatusBadRequest, "workspace name, provider instance, repository, and ref are required")
		return
	}
	workspace := models.WorkspaceConfig{ID: stableCloudUserID(session.User.ID + ":snapshot:" + input.ProviderInstanceID + ":" + input.ProviderRepository), Name: name, AccessMode: models.WorkspaceAccessModeRemoteSnapshot, OwnerUserID: session.User.ID, Provider: strings.TrimSpace(input.Provider), ProviderInstanceID: strings.TrimSpace(input.ProviderInstanceID), ProviderRepository: strings.TrimSpace(input.ProviderRepository), SelectedRef: strings.TrimSpace(input.SelectedRef), BaselineBranch: strings.TrimSpace(input.SelectedRef), Sources: normalizeCloudSources(input.Sources), CreatedAt: time.Now().UTC(), LastScannedAt: time.Now().UTC()}
	adapter := remoteSnapshotAdapter{providers: a.cloudProviders}
	resolved, _, err := adapter.Resolve(r.Context(), session, workspace)
	if err != nil {
		writeError(w, http.StatusBadRequest, "provider repository or ref is not available")
		return
	}
	created, err := a.cloudWorkspaces.Upsert(r.Context(), resolved)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Cloud workspace persistence is unavailable")
		return
	}
	a.audit.recordOwned(session.User.ID, session.User.ID, created.ID, "cloud_snapshot_create", "Remote snapshot workspace created")
	writeJSON(w, http.StatusCreated, created)
}
