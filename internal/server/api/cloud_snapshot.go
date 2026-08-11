package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path"
	"strings"

	"kode-stream/internal/common/models"
	workspacecap "kode-stream/internal/workspace"
)

type remoteSnapshotView struct {
	Workspace    models.WorkspaceConfig                             `json:"workspace"`
	Capabilities map[string]bool                                    `json:"capabilities"`
	Actions      map[models.WorkspaceAction]models.ActionCapability `json:"actions"`
}

func (a *cloudController) remoteSnapshot(r *http.Request) (remoteSnapshotAdapter, cloudSession, models.WorkspaceConfig, int, string) {
	session, ok := cloudSessionFromContext(r.Context())
	if !ok {
		return remoteSnapshotAdapter{}, cloudSession{}, models.WorkspaceConfig{}, http.StatusUnauthorized, "Cloud session is required"
	}
	workspace, ok, err := a.workspaces.Get(r.Context(), session.User.ID, r.PathValue("id"))
	if err != nil {
		return remoteSnapshotAdapter{}, cloudSession{}, models.WorkspaceConfig{}, http.StatusServiceUnavailable, "Cloud workspace persistence is unavailable"
	}
	if !ok {
		return remoteSnapshotAdapter{}, cloudSession{}, models.WorkspaceConfig{}, http.StatusNotFound, "workspace not found"
	}
	if workspace.AccessMode != models.WorkspaceAccessModeRemoteSnapshot {
		return remoteSnapshotAdapter{}, cloudSession{}, models.WorkspaceConfig{}, http.StatusConflict, "workspace is not a remote snapshot"
	}
	return remoteSnapshotAdapter{providers: a.providers}, session, workspace, 0, ""
}

func snapshotCapabilities() map[string]bool {
	return map[string]bool{"read": true, "snapshot_selection": true, "terminal_handoff": true, "write": false, "git": false, "terminal": false, "ai": false, "runtime": false, "verification": false}
}

func snapshotActionCapabilities(role models.CloudRole, workspace models.WorkspaceConfig) map[models.WorkspaceAction]models.ActionCapability {
	return workspacecap.ResolveActionCapabilities(workspacecap.ActionCapabilityInput{
		Axes:               workspacecap.ProviderAxes(models.RuntimeModeCloud, models.AppStateDatastorePostgres, workspace),
		Authorization:      roleCapabilities(role),
		ContentAvailable:   true,
		ExecutionAvailable: false,
		Writable:           false,
	})
}

func (a *cloudController) cloudSnapshotInfo(w http.ResponseWriter, r *http.Request) {
	adapter, session, workspace, status, message := a.remoteSnapshot(r)
	if message != "" {
		writeError(w, status, message)
		return
	}
	resolved, _, err := adapter.Resolve(r.Context(), session, workspace)
	if err != nil {
		writeError(w, http.StatusBadGateway, "remote snapshot is unavailable")
		return
	}
	if _, err := a.workspaces.Upsert(r.Context(), resolved); err != nil {
		writeError(w, http.StatusServiceUnavailable, "Cloud workspace persistence is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, remoteSnapshotView{Workspace: resolved, Capabilities: snapshotCapabilities(), Actions: snapshotActionCapabilities(session.User.Role, resolved)})
}

func (a *cloudController) cloudSnapshotTree(w http.ResponseWriter, r *http.Request) {
	adapter, session, workspace, status, message := a.remoteSnapshot(r)
	if message != "" {
		writeError(w, status, message)
		return
	}
	resolved, integration, err := adapter.Resolve(r.Context(), session, workspace)
	if err != nil {
		writeError(w, http.StatusBadGateway, "remote snapshot is unavailable")
		return
	}
	directory := cleanSnapshotPath(r.URL.Query().Get("path"))
	entries, err := integration.Tree(r.Context(), resolved.ProviderRepository, resolved.ResolvedCommitSHA, directory)
	if err != nil {
		writeError(w, http.StatusBadGateway, "remote snapshot tree is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaceId": resolved.ID, "commitSha": resolved.ResolvedCommitSHA, "entries": entries})
}

func (a *cloudController) cloudSnapshotFile(w http.ResponseWriter, r *http.Request) {
	adapter, session, workspace, status, message := a.remoteSnapshot(r)
	if message != "" {
		writeError(w, status, message)
		return
	}
	filePath := cleanSnapshotPath(r.URL.Query().Get("path"))
	if filePath == "" {
		writeError(w, http.StatusBadRequest, "snapshot file path is required")
		return
	}
	resolved, integration, err := adapter.Resolve(r.Context(), session, workspace)
	if err != nil {
		writeError(w, http.StatusBadGateway, "remote snapshot is unavailable")
		return
	}
	file, err := integration.ReadFile(r.Context(), resolved.ProviderRepository, resolved.ResolvedCommitSHA, filePath)
	if err != nil {
		writeError(w, http.StatusBadGateway, "remote snapshot file is unavailable")
		return
	}
	digest := sha256.Sum256([]byte(file.Content))
	writeJSON(w, http.StatusOK, map[string]any{"path": file.Path, "content": file.Content, "hash": hex.EncodeToString(digest[:]), "commitSha": resolved.ResolvedCommitSHA, "editable": false})
}

func cleanSnapshotPath(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" || value == "." || strings.HasPrefix(value, "../") || strings.Contains(value, "/../") {
		return ""
	}
	return strings.Trim(path.Clean(value), "/")
}
