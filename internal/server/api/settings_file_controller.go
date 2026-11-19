package api

import (
	"errors"
	"net/http"

	"kode-stream/internal/common/models"
	"kode-stream/internal/workspace/settingsfile"
)

// settingsFileView is what Settings shows for a workspace's shared file:
// whether its checkout has one, what it holds, and what an export of the
// workspace's settings now would write.
type settingsFileView struct {
	Path    string             `json:"path"`
	Exists  bool               `json:"exists"`
	Error   string             `json:"error,omitempty"`
	File    *settingsfile.File `json:"file,omitempty"`
	Has     []string           `json:"has"`
	Current settingsfile.File  `json:"current"`
	Parts   []string           `json:"parts"`
}

type settingsFileRequest struct {
	Parts []string `json:"parts"`
}

func (a *harnessController) settingsFileWorkspace(w http.ResponseWriter, r *http.Request) (models.WorkspaceConfig, bool) {
	if a.workspaces == nil {
		writeError(w, http.StatusServiceUnavailable, "workspaces are unavailable")
		return models.WorkspaceConfig{}, false
	}
	workspace, found, err := a.workspaces.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return workspace, false
	}
	if !found || workspace.Path == "" {
		writeError(w, http.StatusNotFound, "workspace not found")
		return workspace, false
	}
	return workspace, true
}

// currentSettingsFile is the workspace's settings as an export writes them.
func (a *harnessController) currentSettingsFile(r *http.Request, workspaceID string) (settingsfile.File, error) {
	harnessPart, err := a.harness.WorkspaceFile(r.Context(), workspaceID)
	if err != nil {
		return settingsfile.File{}, err
	}
	file := settingsfile.File{Version: 1, Harness: &harnessPart}
	if a.ai != nil && a.ai.aiSessions != nil {
		if settings, err := a.ai.aiSessions.Settings(); err == nil && settings.ProviderFor(workspaceID) != "" {
			file.AI = &settingsfile.AI{Provider: settings.ProviderFor(workspaceID)}
		}
	}
	return file, nil
}

func (a *harnessController) settingsFile(w http.ResponseWriter, r *http.Request) {
	workspace, ok := a.settingsFileWorkspace(w, r)
	if !ok {
		return
	}
	current, err := a.currentSettingsFile(r, workspace.ID)
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	view := settingsFileView{Path: settingsfile.Path, Has: []string{}, Current: current, Parts: settingsfile.Parts}
	file, exists, err := settingsfile.Load(workspace.Path)
	view.Exists = exists
	if err != nil {
		view.Error = err.Error()
	} else if exists {
		view.File, view.Has = &file, file.Has()
	}
	writeJSON(w, http.StatusOK, view)
}

func (a *harnessController) exportSettingsFile(w http.ResponseWriter, r *http.Request) {
	workspace, ok := a.settingsFileWorkspace(w, r)
	if !ok {
		return
	}
	var request settingsFileRequest
	if !decodeLimitedJSON(w, r, &request, 16<<10, true) {
		return
	}
	if err := settingsfile.ValidParts(request.Parts); err != nil {
		writeHarnessInvalid(w, err.Error())
		return
	}
	current, err := a.currentSettingsFile(r, workspace.ID)
	if err != nil {
		writeHarnessError(w, err)
		return
	}
	existing, _, err := settingsfile.Load(workspace.Path)
	if err != nil {
		writeHarnessInvalid(w, "The file in the repository is not valid, so it was not overwritten: "+err.Error())
		return
	}
	file := settingsfile.Take(existing, current, request.Parts)
	if err := settingsfile.Write(workspace.Path, file); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": settingsfile.Path, "has": file.Has()})
}

func (a *harnessController) importSettingsFile(w http.ResponseWriter, r *http.Request) {
	workspace, ok := a.settingsFileWorkspace(w, r)
	if !ok {
		return
	}
	var request settingsFileRequest
	if !decodeLimitedJSON(w, r, &request, 16<<10, true) {
		return
	}
	if err := settingsfile.ValidParts(request.Parts); err != nil {
		writeHarnessInvalid(w, err.Error())
		return
	}
	file, exists, err := settingsfile.Load(workspace.Path)
	if err == nil && !exists {
		err = errors.New("this workspace has no " + settingsfile.Path)
	}
	if err != nil {
		writeHarnessInvalid(w, err.Error())
		return
	}
	has := map[string]bool{}
	for _, part := range file.Has() {
		has[part] = true
	}
	harnessParts := []string{}
	for _, part := range request.Parts {
		if !has[part] {
			writeHarnessInvalid(w, "the file holds no "+part)
			return
		}
		if part != settingsfile.PartProvider {
			harnessParts = append(harnessParts, part)
		}
	}
	if has[settingsfile.PartProvider] && contains(request.Parts, settingsfile.PartProvider) {
		if a.ai == nil || a.ai.aiSessions == nil {
			writeError(w, http.StatusServiceUnavailable, "AI session settings are unavailable")
			return
		}
		settings, err := a.ai.aiSessions.Settings()
		if err == nil {
			if settings.WorkspaceProviders == nil {
				settings.WorkspaceProviders = map[string]string{}
			}
			settings.WorkspaceProviders[workspace.ID] = file.AI.Provider
			_, err = a.ai.aiSessions.Save(settings)
		}
		if err != nil {
			writeHarnessInvalid(w, err.Error())
			return
		}
	}
	if len(harnessParts) > 0 {
		if err := a.harness.ImportWorkspaceFile(r.Context(), workspace.ID, *file.Harness, harnessParts, harnessActor(r)); err != nil {
			writeHarnessError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": request.Parts})
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
