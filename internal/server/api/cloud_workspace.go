package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"kode-stream/internal/common/models"
)

type cloudWorkspaceStore struct {
	mu         sync.RWMutex
	workspaces map[string]map[string]models.WorkspaceConfig
}

func newCloudWorkspaceStore() *cloudWorkspaceStore {
	return &cloudWorkspaceStore{workspaces: map[string]map[string]models.WorkspaceConfig{}}
}

func (s *cloudWorkspaceStore) List(userID string) []models.WorkspaceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	owned := s.workspaces[userID]
	if len(owned) == 0 {
		return []models.WorkspaceConfig{}
	}
	result := make([]models.WorkspaceConfig, 0, len(owned))
	for _, workspace := range owned {
		result = append(result, workspace)
	}
	return result
}

func (s *cloudWorkspaceStore) Get(userID, workspaceID string) (models.WorkspaceConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspace, ok := s.workspaces[userID][workspaceID]
	return workspace, ok
}

func (s *cloudWorkspaceStore) Upsert(workspace models.WorkspaceConfig) models.WorkspaceConfig {
	workspace = normalizeCloudWorkspaceAccess(workspace)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workspaces[workspace.OwnerUserID] == nil {
		s.workspaces[workspace.OwnerUserID] = map[string]models.WorkspaceConfig{}
	}
	s.workspaces[workspace.OwnerUserID][workspace.ID] = workspace
	return workspace
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

func (a *API) registerCloudWorkspaceFromAgent(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusCreated, a.cloudWorkspaces.Upsert(workspace))
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

func (a *API) rejectCloudBrowserWorkspaceRegistration(w http.ResponseWriter, input models.WorkspaceInput) bool {
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

func (a *API) createCloudRemoteSnapshotWorkspace(w http.ResponseWriter, r *http.Request, input models.WorkspaceInput) {
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
	writeJSON(w, http.StatusCreated, a.cloudWorkspaces.Upsert(resolved))
}
