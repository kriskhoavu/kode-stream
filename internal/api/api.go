package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"plan-manager/internal/application/apperrors"
	appgit "plan-manager/internal/application/git"
	appitem "plan-manager/internal/application/item"
	appworkspace "plan-manager/internal/application/workspace"
	"plan-manager/internal/fileaccess"
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/itemindex"
	"plan-manager/internal/itemwriter"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
	"plan-manager/internal/scanner"
	"plan-manager/internal/systemdialog"
)

type API struct {
	workspaces *appworkspace.Service
	items      *appitem.Service
	gitOps     *appgit.Service
	dialog     *systemdialog.Dialog
}

func New(reg *registry.Registry, idx *itemindex.Index, scan *scanner.Scanner, files *fileaccess.Access, writer *itemwriter.Writer, git *gitadapter.GitAdapter, dialog *systemdialog.Dialog) *API {
	return &API{
		workspaces: appworkspace.New(reg, idx, scan, writer),
		items:      appitem.New(reg, idx, files, writer, git),
		gitOps:     appgit.New(reg, writer, git),
		dialog:     dialog,
	}
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/state", a.state)
	mux.HandleFunc("GET /api/workspaces", a.listWorkspaces)
	mux.HandleFunc("POST /api/workspaces", a.createWorkspace)
	mux.HandleFunc("PUT /api/workspaces/{id}", a.updateWorkspace)
	mux.HandleFunc("DELETE /api/workspaces/{id}", a.deleteWorkspace)
	mux.HandleFunc("POST /api/workspaces/{id}/scan", a.scanWorkspace)
	mux.HandleFunc("GET /api/workspaces/{id}/source-structure", a.getSourceStructure)
	mux.HandleFunc("PUT /api/workspaces/{id}/source-structure", a.saveSourceStructure)
	mux.HandleFunc("GET /api/items", a.listItems)
	mux.HandleFunc("GET /api/items/{id}", a.itemDetail)
	mux.HandleFunc("GET /api/items/{id}/files", a.itemFiles)
	mux.HandleFunc("GET /api/items/{id}/files/{fileID}", a.itemFileContent)
	mux.HandleFunc("POST /api/items/{id}/files/{fileID}", a.saveItemFile)
	mux.HandleFunc("POST /api/items/{id}/files/{fileID}/revert", a.revertItemFile)
	mux.HandleFunc("GET /api/items/{id}/diff", a.itemDiff)
	mux.HandleFunc("PATCH /api/items/{id}/metadata", a.saveItemMetadata)
	mux.HandleFunc("PATCH /api/items/{id}/status", a.updateItemStatus)
	mux.HandleFunc("POST /api/items", a.createItem)
	mux.HandleFunc("GET /api/workspaces/{id}/git/status", a.gitStatus)
	mux.HandleFunc("POST /api/workspaces/{id}/git/fetch", a.gitFetch)
	mux.HandleFunc("POST /api/workspaces/{id}/git/pull", a.gitPull)
	mux.HandleFunc("POST /api/workspaces/{id}/git/push", a.gitPush)
	mux.HandleFunc("POST /api/workspaces/{id}/git/commit", a.gitCommit)
	mux.HandleFunc("POST /api/workspaces/{id}/git/branches", a.gitCreateBranch)
	mux.HandleFunc("POST /api/workspaces/{id}/git/switch", a.gitSwitchBranch)
	mux.HandleFunc("POST /api/system/select-directory", a.selectDirectory)
	mux.HandleFunc("POST /api/system/open-path", a.openPath)
	return mux
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) state(w http.ResponseWriter, r *http.Request) {
	state, err := a.workspaces.State()
	respond(w, state, err)
}

func (a *API) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := a.workspaces.List()
	respond(w, workspaces, err)
}

func (a *API) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	workspace, err := a.workspaces.Create(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, workspace)
}

func (a *API) updateWorkspace(w http.ResponseWriter, r *http.Request) {
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

func (a *API) deleteWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.workspaces.Delete(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) scanWorkspace(w http.ResponseWriter, r *http.Request) {
	result, err := a.workspaces.Scan(r.PathValue("id"))
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

func (a *API) getSourceStructure(w http.ResponseWriter, r *http.Request) {
	result, err := a.workspaces.SourceStructure(r.PathValue("id"), r.URL.Query().Get("directory"))
	respondWorkspaceResult(w, result, err)
}

func (a *API) saveSourceStructure(w http.ResponseWriter, r *http.Request) {
	var settings models.SourceStructureSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.workspaces.SaveSourceStructure(r.PathValue("id"), r.URL.Query().Get("directory"), settings)
	respondWorkspaceResult(w, result, err)
}

func (a *API) listItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, err := a.items.List(appitem.ListInput{
		WorkspaceID: q.Get("workspaceId"),
		Branch:      q.Get("branch"),
		Status:      q.Get("status"),
		Text:        q.Get("q"),
	})
	respond(w, items, err)
}

func (a *API) itemDetail(w http.ResponseWriter, r *http.Request) {
	item, err := a.items.Detail(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	respond(w, item, err)
}

func (a *API) itemFiles(w http.ResponseWriter, r *http.Request) {
	tree, err := a.items.Files(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	respond(w, tree, err)
}

func (a *API) itemFileContent(w http.ResponseWriter, r *http.Request) {
	content, err := a.items.FileContent(r.PathValue("id"), r.PathValue("fileID"))
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	respond(w, content, err)
}

func (a *API) itemDiff(w http.ResponseWriter, r *http.Request) {
	diff, err := a.items.Diff(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": diff})
}

func (a *API) saveItemFile(w http.ResponseWriter, r *http.Request) {
	if _, err := a.items.Detail(r.PathValue("id")); errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	var input models.FileSaveInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.items.SaveFile(r.PathValue("id"), r.PathValue("fileID"), input)
	respond(w, result, err)
}

func (a *API) revertItemFile(w http.ResponseWriter, r *http.Request) {
	result, err := a.items.RevertFile(r.PathValue("id"), r.PathValue("fileID"), validateGitPaths)
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	respond(w, result, err)
}

func (a *API) saveItemMetadata(w http.ResponseWriter, r *http.Request) {
	var input models.ItemMetadataUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.items.SaveMetadata(r.PathValue("id"), input)
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	respond(w, result, err)
}

func (a *API) updateItemStatus(w http.ResponseWriter, r *http.Request) {
	var input models.ItemStatusUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.items.UpdateStatus(r.PathValue("id"), input)
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	respond(w, result, err)
}

func (a *API) createItem(w http.ResponseWriter, r *http.Request) {
	var input models.NewItemInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.items.Create(input)
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, result, err)
}

func (a *API) gitStatus(w http.ResponseWriter, r *http.Request) {
	status, err := a.gitOps.Status(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, status, err)
}

func (a *API) gitFetch(w http.ResponseWriter, r *http.Request) {
	a.gitOperation(w, r, a.gitOps.Fetch)
}

func (a *API) gitPull(w http.ResponseWriter, r *http.Request) {
	a.gitOperation(w, r, a.gitOps.Pull)
}

func (a *API) gitPush(w http.ResponseWriter, r *http.Request) {
	a.gitOperation(w, r, a.gitOps.Push)
}

func (a *API) gitCommit(w http.ResponseWriter, r *http.Request) {
	var input models.GitCommitInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	respondGitResult(w, a.gitOps.Commit(r.PathValue("id"), input))
}

func (a *API) gitCreateBranch(w http.ResponseWriter, r *http.Request) {
	var input models.BranchCreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	respondGitResult(w, a.gitOps.CreateBranch(r.PathValue("id"), input))
}

func (a *API) gitSwitchBranch(w http.ResponseWriter, r *http.Request) {
	var input models.BranchSwitchInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	respondGitResult(w, a.gitOps.SwitchBranch(r.PathValue("id"), input))
}

func (a *API) gitOperation(w http.ResponseWriter, r *http.Request, run func(string, models.GitOperationInput) models.GitOperationResult) {
	var input models.GitOperationInput
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}
	respondGitResult(w, run(r.PathValue("id"), input))
}

func (a *API) selectDirectory(w http.ResponseWriter, r *http.Request) {
	path, err := a.dialog.SelectDirectory()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

func (a *API) openPath(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := a.dialog.OpenPath(input.Path); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func nonNilWarnings(warnings []models.ScanWarning) []models.ScanWarning {
	return appworkspace.NonNilWarnings(warnings)
}

func validateGitPaths(workspace models.WorkspaceConfig, paths []string) error {
	return appgit.ValidatePaths(workspace, paths)
}

func statusForError(err error) int {
	if err != nil {
		return http.StatusBadRequest
	}
	return http.StatusOK
}

func fallbackItemPath(workspace models.WorkspaceConfig, item models.ItemDetail) string {
	return appitem.FallbackPath(workspace, item)
}

func fullReadmeDescription(workspace models.WorkspaceConfig, item models.ItemDetail) string {
	return appitem.FullReadmeDescription(workspace, item)
}

func normalizeItemSummary(item models.ItemSummary) models.ItemSummary {
	return appitem.NormalizeSummary(item)
}

func normalizeItemDetail(item models.ItemDetail) models.ItemDetail {
	return appitem.NormalizeDetail(item)
}

func firstMarkdownParagraph(markdown string) string {
	return appitem.FirstMarkdownParagraph(markdown)
}

func respond(w http.ResponseWriter, data any, err error) {
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func respondWorkspaceResult(w http.ResponseWriter, data any, err error) {
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, data, err)
}

func respondGitResult(w http.ResponseWriter, result models.GitOperationResult) {
	if result.Message == apperrors.ErrWorkspaceNotFound.Error() {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	writeJSON(w, statusForErrorFromResult(result), result)
}

func statusForErrorFromResult(result models.GitOperationResult) int {
	if !result.OK {
		return http.StatusBadRequest
	}
	return http.StatusOK
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s\n", r.Method, r.URL.Path)
	})
}
