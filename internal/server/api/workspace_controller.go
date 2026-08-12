package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	appsearch "kode-stream/internal/search"
	"kode-stream/internal/system"
	appworkspace "kode-stream/internal/workspace"
	workspacehealth "kode-stream/internal/workspace"
	appworkstream "kode-stream/internal/workstream"
)

type workspaceController struct {
	workspaces      *appworkspace.Service
	workstream      *appworkstream.Service
	health          *workspacehealth.HealthService
	files           *appworkspace.WorkspaceFileService
	contentSearch   *appsearch.ContentSearchService
	runtimeConfig   system.RuntimeConfig
	cloudWorkspaces *cloudWorkspaceStore
	cloudProviders  *cloudProviderStore
	audit           auditRecorder
}

func (a *workspaceController) previewWorkspaceImport(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SourcePath string `json:"sourcePath"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	preview, err := a.workspaces.PreviewImport(input.SourcePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (a *workspaceController) importWorkspaces(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceImportRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	results, err := a.workspaces.Import(input)
	if err != nil {
		if errors.Is(err, appworkspace.ErrStaleImportPreview) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error(), "code": "stale_import_preview"})
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (a *workspaceController) workspaceHealth(w http.ResponseWriter, r *http.Request) {
	if a.health == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace health is unavailable")
		return
	}
	result, err := a.health.Check(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, result, err)
}

func (a *workspaceController) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	if a.runtimeConfig.Mode == models.RuntimeModeCloud {
		session, ok := cloudSessionFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "Cloud session is required")
			return
		}
		workspaces, err := a.cloudWorkspaces.List(r.Context(), session.User.ID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "Cloud workspace persistence is unavailable")
			return
		}
		writeJSON(w, http.StatusOK, workspaces)
		return
	}
	workspaces, err := a.workspaces.List()
	respond(w, workspaces, err)
}

func (a *workspaceController) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if a.runtimeConfig.Mode == models.RuntimeModeCloud {
		a.createCloudRemoteSnapshotWorkspace(w, r, input)
		return
	}
	result, err := a.workspaces.CreateWithResult(input)
	if err != nil {
		if strings.TrimSpace(result.OperationLog) != "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "operationLog": result.OperationLog})
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(result.OperationLog) == "" {
		writeJSON(w, http.StatusCreated, result.Workspace)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *workspaceController) createWorkspaceStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	var input models.WorkspaceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if a.rejectCloudBrowserWorkspaceRegistration(w, input) {
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	writeSSE(w, "start", map[string]any{"ok": true})
	flusher.Flush()
	result, err := a.workspaces.CreateWithResultStreaming(input, func(chunk string) {
		if strings.TrimSpace(chunk) == "" {
			return
		}
		writeSSE(w, "log", map[string]any{"chunk": chunk})
		flusher.Flush()
	})
	if err != nil {
		writeSSE(w, "error", map[string]any{"error": err.Error(), "operationLog": result.OperationLog})
		flusher.Flush()
		return
	}
	writeSSE(w, "result", result)
	flusher.Flush()
}

func (a *workspaceController) updateWorkspace(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	workspace, err := a.workspaces.Update(r.PathValue("id"), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workspace)
}

func (a *workspaceController) deleteWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.workspaces.Delete(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *workspaceController) scanWorkspace(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	result, err := a.workspaces.ScanContext(r.Context(), r.PathValue("id"))
	a.audit.record(r.PathValue("id"), "", "scan", "Workspace scan completed.", nil, started, err)
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *workspaceController) loadWorkstreamBranch(w http.ResponseWriter, r *http.Request) {
	var input models.WorkstreamBranchLoadInput
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	started := time.Now()
	result, err := a.workstream.LoadBranchContext(r.Context(), r.PathValue("id"), input)
	a.audit.record(r.PathValue("id"), "", "workstream_branch_load", "Workstream branch loaded.", nil, started, err)
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	if errors.Is(err, appworkstream.ErrBranchReviewRequired) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "branch_review_required"})
		return
	}
	respond(w, result, err)
}

func (a *workspaceController) loadWorkstreamCheckout(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Force bool `json:"force,omitempty"`
	}
	if r.Body != nil {
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	result, err := a.workstream.LoadCheckoutContext(r.Context(), r.PathValue("id"), input.Force)
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, result, err)
}

func (a *workspaceController) loadBranchReview(w http.ResponseWriter, r *http.Request) {
	var input models.WorkstreamBranchLoadInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.workstream.ReviewBranchContext(r.Context(), r.PathValue("id"), input)
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	if errors.Is(err, appworkstream.ErrReviewMatchesCheckout) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "review_matches_checkout"})
		return
	}
	respond(w, result, err)
}

func (a *workspaceController) getSourceStructure(w http.ResponseWriter, r *http.Request) {
	result, err := a.workspaces.SourceStructure(r.PathValue("id"), r.URL.Query().Get("directory"))
	respondWorkspaceResult(w, result, err)
}

func (a *workspaceController) saveSourceStructure(w http.ResponseWriter, r *http.Request) {
	var settings models.SourceStructureSettings
	if !decodeLimitedJSON(w, r, &settings, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	result, err := a.workspaces.SaveSourceStructure(r.PathValue("id"), r.URL.Query().Get("directory"), settings)
	respondWorkspaceResult(w, result, err)
}

func (a *workspaceController) resetSourceStructure(w http.ResponseWriter, r *http.Request) {
	result, err := a.workspaces.ResetSourceStructure(r.PathValue("id"), r.URL.Query().Get("directory"))
	respondWorkspaceResult(w, result, err)
}

func (a *workspaceController) workspaceTree(w http.ResponseWriter, r *http.Request) {
	includeIgnored, _ := strconv.ParseBool(r.URL.Query().Get("includeIgnored"))
	result, err := a.files.List(r.PathValue("id"), r.URL.Query().Get("path"), includeIgnored)
	respondWorkspaceFileResult(w, result, err)
}

func (a *workspaceController) workspacePathSearch(w http.ResponseWriter, r *http.Request) {
	includeIgnored, _ := strconv.ParseBool(r.URL.Query().Get("includeIgnored"))
	result, err := a.files.SearchContext(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("workspaceId"), includeIgnored)
	respondSearch(w, result, err)
}

func (a *workspaceController) workspaceContentSearch(w http.ResponseWriter, r *http.Request) {
	includeIgnored, err := optionalBool(r, "includeIgnored")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	caseSensitive, err := optionalBool(r, "caseSensitive")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := a.contentSearch.SearchExplorer(r.Context(), r.URL.Query().Get("mode"), r.URL.Query().Get("workspaceId"), models.WorkspaceContentSearchRequest{
		Query: r.URL.Query().Get("q"), IncludeIgnored: includeIgnored, CaseSensitive: caseSensitive,
	})
	respondContentSearch(w, result, err)
}

func (a *workspaceController) workspaceFile(w http.ResponseWriter, r *http.Request) {
	result, err := a.files.Read(r.PathValue("id"), r.URL.Query().Get("path"))
	respondWorkspaceFileResult(w, result, err)
}

func (a *workspaceController) saveWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceFileSaveInput
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	result, err := a.files.Save(r.PathValue("id"), input)
	respondWorkspaceFileResult(w, result, err)
}

func (a *workspaceController) createWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceFileCreateInput
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	result, err := a.files.CreateFile(r.PathValue("id"), input)
	respondWorkspaceFileResult(w, result, err)
}

func (a *workspaceController) createWorkspaceDirectory(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceDirectoryCreateInput
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	result, err := a.files.CreateDirectory(r.PathValue("id"), input)
	respondWorkspaceFileResult(w, result, err)
}

func (a *workspaceController) renameWorkspacePath(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspacePathRenameInput
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	result, err := a.files.Rename(r.PathValue("id"), input)
	respondWorkspaceFileResult(w, result, err)
}

func (a *workspaceController) workspacePathGitStates(w http.ResponseWriter, r *http.Request) {
	result, err := a.files.PathStates(r.PathValue("id"))
	respondWorkspaceFileResult(w, result, err)
}

func (a *workspaceController) workspaceFileDiff(w http.ResponseWriter, r *http.Request) {
	diff, err := a.files.DiffContext(r.Context(), r.PathValue("id"), r.URL.Query().Get("path"))
	if err != nil {
		respondWorkspaceFileResult(w, nil, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": diff})
}

func (a *workspaceController) revertWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceFileRevertInput
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	result, err := a.files.RevertContext(r.Context(), r.PathValue("id"), input)
	respondWorkspaceFileResult(w, result, err)
}
