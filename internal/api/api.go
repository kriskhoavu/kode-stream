package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"plan-manager/internal/application/apperrors"
	appcontentsearch "plan-manager/internal/application/contentsearch"
	appgit "plan-manager/internal/application/git"
	apphealth "plan-manager/internal/application/health"
	appitem "plan-manager/internal/application/item"
	appsearch "plan-manager/internal/application/search"
	appworkspace "plan-manager/internal/application/workspace"
	appworkspacefiles "plan-manager/internal/application/workspacefiles"
	"plan-manager/internal/audit"
	"plan-manager/internal/config"
	"plan-manager/internal/fileaccess"
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/itemindex"
	"plan-manager/internal/itemwriter"
	"plan-manager/internal/models"
	"plan-manager/internal/navigation"
	"plan-manager/internal/registry"
	"plan-manager/internal/scanner"
	"plan-manager/internal/systemdialog"
	workspaceaccess "plan-manager/internal/workspacefiles"
)

type API struct {
	workspaces     *appworkspace.Service
	items          *appitem.Service
	gitOps         *appgit.Service
	dialog         *systemdialog.Dialog
	audit          *audit.Store
	healthService  *apphealth.Service
	search         *appsearch.Service
	navigation     *navigation.Store
	workspaceFiles *appworkspacefiles.Service
	contentSearch  *appcontentsearch.Service
}

func New(reg *registry.Registry, idx *itemindex.Index, scan *scanner.Scanner, files *fileaccess.Access, writer *itemwriter.Writer, git *gitadapter.GitAdapter, dialog *systemdialog.Dialog) *API {
	return NewWithReliability(reg, idx, scan, files, writer, git, dialog, nil, nil)
}

func NewWithReliability(reg *registry.Registry, idx *itemindex.Index, scan *scanner.Scanner, files *fileaccess.Access, writer *itemwriter.Writer, git *gitadapter.GitAdapter, dialog *systemdialog.Dialog, auditStore *audit.Store, healthService *apphealth.Service) *API {
	return NewWithServices(reg, idx, scan, files, writer, git, dialog, auditStore, healthService, nil, nil)
}

func NewWithServices(reg *registry.Registry, idx *itemindex.Index, scan *scanner.Scanner, files *fileaccess.Access, writer *itemwriter.Writer, git *gitadapter.GitAdapter, dialog *systemdialog.Dialog, auditStore *audit.Store, healthService *apphealth.Service, searchService *appsearch.Service, navigationStore *navigation.Store) *API {
	var refresher appworkspacefiles.Refresher
	if writer != nil {
		refresher = writer
	}
	workspaceFileAccess := workspaceaccess.New()
	return &API{
		workspaces:     appworkspace.New(reg, idx, scan, writer, git),
		items:          appitem.New(reg, idx, files, writer, git),
		gitOps:         appgit.New(reg, writer, git),
		dialog:         dialog,
		audit:          auditStore,
		healthService:  healthService,
		search:         searchService,
		navigation:     navigationStore,
		workspaceFiles: appworkspacefiles.New(reg, workspaceFileAccess, git, auditStore, refresher),
		contentSearch:  appcontentsearch.New(reg, idx, workspaceFileAccess),
	}
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/state", a.state)
	mux.HandleFunc("GET /api/audit-events", a.auditEvents)
	mux.HandleFunc("GET /api/search", a.searchItems)
	mux.HandleFunc("GET /api/saved-filters", a.savedFilters)
	mux.HandleFunc("POST /api/saved-filters", a.saveFilter)
	mux.HandleFunc("DELETE /api/saved-filters/{id}", a.deleteFilter)
	mux.HandleFunc("GET /api/recent-items", a.recentItems)
	mux.HandleFunc("POST /api/recent-items", a.recordRecentItem)
	mux.HandleFunc("GET /api/workspaces", a.listWorkspaces)
	mux.HandleFunc("POST /api/workspaces", a.createWorkspace)
	mux.HandleFunc("POST /api/workspaces/stream-create", a.createWorkspaceStream)
	mux.HandleFunc("PUT /api/workspaces/{id}", a.updateWorkspace)
	mux.HandleFunc("DELETE /api/workspaces/{id}", a.deleteWorkspace)
	mux.HandleFunc("POST /api/workspaces/{id}/scan", a.scanWorkspace)
	mux.HandleFunc("POST /api/workspaces/{id}/kanban/branch", a.loadKanbanBranch)
	mux.HandleFunc("GET /api/workspaces/{id}/health", a.workspaceHealth)
	mux.HandleFunc("GET /api/workspaces/{id}/source-structure", a.getSourceStructure)
	mux.HandleFunc("PUT /api/workspaces/{id}/source-structure", a.saveSourceStructure)
	mux.HandleFunc("DELETE /api/workspaces/{id}/source-structure", a.resetSourceStructure)
	mux.HandleFunc("GET /api/workspaces/{id}/tree", a.workspaceTree)
	mux.HandleFunc("GET /api/workspaces/files/search", a.workspacePathSearch)
	mux.HandleFunc("GET /api/workspaces/files/content-search", a.workspaceContentSearch)
	mux.HandleFunc("GET /api/workspaces/{id}/files", a.workspaceFile)
	mux.HandleFunc("PUT /api/workspaces/{id}/files", a.saveWorkspaceFile)
	mux.HandleFunc("POST /api/workspaces/{id}/files", a.createWorkspaceFile)
	mux.HandleFunc("POST /api/workspaces/{id}/directories", a.createWorkspaceDirectory)
	mux.HandleFunc("POST /api/workspaces/{id}/paths/rename", a.renameWorkspacePath)
	mux.HandleFunc("GET /api/workspaces/{id}/files/diff", a.workspaceFileDiff)
	mux.HandleFunc("POST /api/workspaces/{id}/files/revert", a.revertWorkspaceFile)
	mux.HandleFunc("GET /api/workspaces/{id}/git/path-status", a.workspacePathGitStates)
	mux.HandleFunc("GET /api/items", a.listItems)
	mux.HandleFunc("GET /api/items/{id}", a.itemDetail)
	mux.HandleFunc("GET /api/items/{id}/files", a.itemFiles)
	mux.HandleFunc("GET /api/items/{id}/content-search", a.itemContentSearch)
	mux.HandleFunc("GET /api/items/{id}/files/{fileID}", a.itemFileContent)
	mux.HandleFunc("POST /api/items/{id}/files/{fileID}", a.saveItemFile)
	mux.HandleFunc("POST /api/items/{id}/files/{fileID}/revert", a.revertItemFile)
	mux.HandleFunc("GET /api/items/{id}/diff", a.itemDiff)
	mux.HandleFunc("PATCH /api/items/{id}/metadata", a.saveItemMetadata)
	mux.HandleFunc("PATCH /api/items/{id}/status", a.updateItemStatus)
	mux.HandleFunc("POST /api/items", a.createItem)
	mux.HandleFunc("GET /api/workspaces/{id}/git/status", a.gitStatus)
	mux.HandleFunc("GET /api/workspaces/{id}/git/activity", a.gitActivity)
	mux.HandleFunc("GET /api/workspaces/{id}/git/branches", a.gitBranches)
	mux.HandleFunc("POST /api/workspaces/{id}/git/fetch", a.gitFetch)
	mux.HandleFunc("POST /api/workspaces/{id}/git/pull", a.gitPull)
	mux.HandleFunc("POST /api/workspaces/{id}/git/push", a.gitPush)
	mux.HandleFunc("POST /api/workspaces/{id}/git/commit", a.gitCommit)
	mux.HandleFunc("POST /api/workspaces/{id}/git/branches", a.gitCreateBranch)
	mux.HandleFunc("POST /api/workspaces/{id}/git/switch", a.gitSwitchBranch)
	mux.HandleFunc("POST /api/system/select-directory", a.selectDirectory)
	mux.HandleFunc("POST /api/system/open-path", a.openPath)
	mux.HandleFunc("GET /api/system/config-paths", a.systemConfigPaths)
	mux.HandleFunc("PUT /api/system/config-paths", a.updateSystemConfigPaths)
	return mux
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) state(w http.ResponseWriter, r *http.Request) {
	state, err := a.workspaces.State()
	respond(w, state, err)
}

func (a *API) auditEvents(w http.ResponseWriter, r *http.Request) {
	if a.audit == nil {
		writeJSON(w, http.StatusOK, []models.AuditEvent{})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	events, err := a.audit.Recent(limit * 2)
	if err != nil {
		respond(w, nil, err)
		return
	}
	workspaceID := r.URL.Query().Get("workspaceId")
	if workspaceID != "" {
		filtered := make([]models.AuditEvent, 0, limit)
		for _, event := range events {
			if event.WorkspaceID == workspaceID {
				filtered = append(filtered, event)
				if len(filtered) == limit {
					break
				}
			}
		}
		events = filtered
	} else if len(events) > limit {
		events = events[:limit]
	}
	writeJSON(w, http.StatusOK, events)
}

func (a *API) searchItems(w http.ResponseWriter, r *http.Request) {
	if a.search == nil {
		writeJSON(w, http.StatusOK, []models.SearchResult{})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	types := strings.Split(strings.TrimSpace(r.URL.Query().Get("types")), ",")
	if len(types) == 1 && types[0] == "" {
		types = nil
	}
	results, err := a.search.Search(models.SearchQuery{Text: r.URL.Query().Get("q"), WorkspaceID: r.URL.Query().Get("workspaceId"), Types: types, Limit: limit})
	respond(w, results, err)
}

func (a *API) savedFilters(w http.ResponseWriter, r *http.Request) {
	if a.navigation == nil {
		writeJSON(w, http.StatusOK, []models.SavedFilter{})
		return
	}
	filters, err := a.navigation.Filters()
	respond(w, filters, err)
}

func (a *API) saveFilter(w http.ResponseWriter, r *http.Request) {
	if a.navigation == nil {
		writeError(w, http.StatusServiceUnavailable, "saved filters are unavailable")
		return
	}
	var filter models.SavedFilter
	if err := json.NewDecoder(r.Body).Decode(&filter); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	filter.Name = strings.TrimSpace(filter.Name)
	if filter.Name == "" {
		writeError(w, http.StatusBadRequest, "saved filter name is required")
		return
	}
	if !validAppRoute(filter.Route) {
		writeError(w, http.StatusBadRequest, "saved filter route is invalid")
		return
	}
	saved, err := a.navigation.SaveFilter(filter)
	if err != nil {
		respond(w, nil, err)
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}

func (a *API) deleteFilter(w http.ResponseWriter, r *http.Request) {
	if a.navigation == nil {
		writeError(w, http.StatusServiceUnavailable, "saved filters are unavailable")
		return
	}
	deleted, err := a.navigation.DeleteFilter(r.PathValue("id"))
	if err != nil {
		respond(w, nil, err)
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "saved filter not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) recentItems(w http.ResponseWriter, r *http.Request) {
	if a.navigation == nil {
		writeJSON(w, http.StatusOK, []models.RecentItem{})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	recents, err := a.navigation.Recents(limit)
	respond(w, recents, err)
}

func (a *API) recordRecentItem(w http.ResponseWriter, r *http.Request) {
	if a.navigation == nil {
		writeError(w, http.StatusServiceUnavailable, "recent items are unavailable")
		return
	}
	var input struct {
		ItemID string `json:"itemId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.ItemID) == "" {
		writeError(w, http.StatusBadRequest, "itemId is required")
		return
	}
	item, err := a.items.Detail(input.ItemID)
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if err != nil {
		respond(w, nil, err)
		return
	}
	recent := models.RecentItem{ItemID: item.ID, WorkspaceID: item.WorkspaceID, Title: item.Title, Subtitle: strings.Trim(strings.Join([]string{item.WorkspaceName, item.Identifier}, " · "), " ·"), Route: "/items/" + url.PathEscape(item.ID)}
	if err := a.navigation.RecordRecent(recent); err != nil {
		respond(w, nil, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func validAppRoute(route string) bool {
	return route == "/kanban" || route == "/items" || route == "/branches" || route == "/workspaces" || strings.HasPrefix(route, "/items/") || strings.HasPrefix(route, "/kanban?")
}

func (a *API) workspaceHealth(w http.ResponseWriter, r *http.Request) {
	if a.healthService == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace health is unavailable")
		return
	}
	result, err := a.healthService.Check(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, result, err)
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

func (a *API) createWorkspaceStream(w http.ResponseWriter, r *http.Request) {
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
	started := time.Now()
	result, err := a.workspaces.Scan(r.PathValue("id"))
	a.record(r.PathValue("id"), "", "scan", "Workspace scan completed.", nil, started, err)
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

func (a *API) loadKanbanBranch(w http.ResponseWriter, r *http.Request) {
	var input models.BranchLoadInput
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	started := time.Now()
	result, err := a.workspaces.LoadBranch(r.PathValue("id"), input)
	a.record(r.PathValue("id"), "", "kanban_branch_load", "Kanban branch loaded.", nil, started, err)
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, result, err)
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

func (a *API) resetSourceStructure(w http.ResponseWriter, r *http.Request) {
	result, err := a.workspaces.ResetSourceStructure(r.PathValue("id"), r.URL.Query().Get("directory"))
	respondWorkspaceResult(w, result, err)
}

func (a *API) workspaceTree(w http.ResponseWriter, r *http.Request) {
	includeIgnored, _ := strconv.ParseBool(r.URL.Query().Get("includeIgnored"))
	result, err := a.workspaceFiles.List(r.PathValue("id"), r.URL.Query().Get("path"), includeIgnored)
	respondWorkspaceFileResult(w, result, err)
}

func (a *API) workspacePathSearch(w http.ResponseWriter, r *http.Request) {
	includeIgnored, _ := strconv.ParseBool(r.URL.Query().Get("includeIgnored"))
	result, err := a.workspaceFiles.Search(r.URL.Query().Get("q"), r.URL.Query().Get("workspaceId"), includeIgnored)
	respondWorkspaceFileResult(w, result, err)
}

func (a *API) workspaceContentSearch(w http.ResponseWriter, r *http.Request) {
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

func (a *API) workspaceFile(w http.ResponseWriter, r *http.Request) {
	result, err := a.workspaceFiles.Read(r.PathValue("id"), r.URL.Query().Get("path"))
	respondWorkspaceFileResult(w, result, err)
}

func (a *API) saveWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceFileSaveInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.workspaceFiles.Save(r.PathValue("id"), input)
	respondWorkspaceFileResult(w, result, err)
}

func (a *API) createWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceFileCreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.workspaceFiles.CreateMarkdown(r.PathValue("id"), input)
	respondWorkspaceFileResult(w, result, err)
}

func (a *API) createWorkspaceDirectory(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceDirectoryCreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.workspaceFiles.CreateDirectory(r.PathValue("id"), input)
	respondWorkspaceFileResult(w, result, err)
}

func (a *API) renameWorkspacePath(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspacePathRenameInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.workspaceFiles.Rename(r.PathValue("id"), input)
	respondWorkspaceFileResult(w, result, err)
}

func (a *API) workspacePathGitStates(w http.ResponseWriter, r *http.Request) {
	result, err := a.workspaceFiles.PathStates(r.PathValue("id"))
	respondWorkspaceFileResult(w, result, err)
}

func (a *API) workspaceFileDiff(w http.ResponseWriter, r *http.Request) {
	diff, err := a.workspaceFiles.Diff(r.PathValue("id"), r.URL.Query().Get("path"))
	if err != nil {
		respondWorkspaceFileResult(w, nil, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": diff})
}

func (a *API) revertWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceFileRevertInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.workspaceFiles.Revert(r.PathValue("id"), input)
	respondWorkspaceFileResult(w, result, err)
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

func (a *API) itemContentSearch(w http.ResponseWriter, r *http.Request) {
	caseSensitive, err := optionalBool(r, "caseSensitive")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := a.contentSearch.SearchItem(r.Context(), r.PathValue("id"), models.WorkspaceContentSearchRequest{
		Query: r.URL.Query().Get("q"), CaseSensitive: caseSensitive,
	})
	respondContentSearch(w, result, err)
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
	item, detailErr := a.items.Detail(r.PathValue("id"))
	if errors.Is(detailErr, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	var input models.FileSaveInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	started := time.Now()
	result, err := a.items.SaveFile(r.PathValue("id"), r.PathValue("fileID"), input)
	a.record(item.WorkspaceID, item.ID, "save_file", "File saved.", []string{result.Path}, started, err)
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
	item, _ := a.items.Detail(r.PathValue("id"))
	started := time.Now()
	result, err := a.items.SaveMetadata(r.PathValue("id"), input)
	a.record(item.WorkspaceID, item.ID, "save_metadata", "Item metadata saved.", []string{item.ItemPath}, started, err)
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
	item, _ := a.items.Detail(r.PathValue("id"))
	started := time.Now()
	result, err := a.items.UpdateStatus(r.PathValue("id"), input)
	a.record(item.WorkspaceID, item.ID, "update_status", "Item status updated.", []string{item.ItemPath}, started, err)
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

func (a *API) gitActivity(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	entries, err := a.gitOps.Activity(r.PathValue("id"), r.URL.Query().Get("path"), limit)
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, entries, err)
}

func (a *API) gitBranches(w http.ResponseWriter, r *http.Request) {
	branches, err := a.gitOps.Branches(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, branches, err)
}

func (a *API) gitFetch(w http.ResponseWriter, r *http.Request) {
	a.gitOperation(w, r, "git_fetch", a.gitOps.Fetch)
}

func (a *API) gitPull(w http.ResponseWriter, r *http.Request) {
	a.gitOperation(w, r, "git_pull", a.gitOps.Pull)
}

func (a *API) gitPush(w http.ResponseWriter, r *http.Request) {
	a.gitOperation(w, r, "git_push", a.gitOps.Push)
}

func (a *API) gitCommit(w http.ResponseWriter, r *http.Request) {
	var input models.GitCommitInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	started := time.Now()
	result := withRecoveryHint(a.gitOps.Commit(r.PathValue("id"), input))
	a.recordGit(r.PathValue("id"), "git_commit", input.Paths, started, result)
	respondGitResult(w, result)
}

func (a *API) gitCreateBranch(w http.ResponseWriter, r *http.Request) {
	var input models.BranchCreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	started := time.Now()
	result := withRecoveryHint(a.gitOps.CreateBranch(r.PathValue("id"), input))
	a.recordGit(r.PathValue("id"), "git_create_branch", nil, started, result)
	respondGitResult(w, result)
}

func (a *API) gitSwitchBranch(w http.ResponseWriter, r *http.Request) {
	var input models.BranchSwitchInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	started := time.Now()
	result := withRecoveryHint(a.gitOps.SwitchBranch(r.PathValue("id"), input))
	a.recordGit(r.PathValue("id"), "git_switch_branch", nil, started, result)
	respondGitResult(w, result)
}

func (a *API) gitOperation(w http.ResponseWriter, r *http.Request, operation string, run func(string, models.GitOperationInput) models.GitOperationResult) {
	var input models.GitOperationInput
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}
	started := time.Now()
	result := withRecoveryHint(run(r.PathValue("id"), input))
	a.recordGit(r.PathValue("id"), operation, nil, started, result)
	respondGitResult(w, result)
}

func (a *API) recordGit(workspaceID, operation string, paths []string, started time.Time, result models.GitOperationResult) {
	var err error
	if !result.OK {
		err = errors.New(result.Message)
	}
	a.record(workspaceID, "", operation, "Git operation completed.", paths, started, err)
}

func (a *API) record(workspaceID, itemID, operation, message string, paths []string, started time.Time, opErr error) {
	if a.audit == nil {
		return
	}
	status := models.AuditStatusSuccess
	errorMessage := ""
	if opErr != nil {
		status = models.AuditStatusFailed
		errorMessage = opErr.Error()
		message = "Operation failed."
		if recoveryHint(errorMessage) != "" {
			status = models.AuditStatusBlocked
			message = "Operation blocked."
		}
	}
	_, _ = a.audit.Append(models.AuditEvent{WorkspaceID: workspaceID, ItemID: itemID, Operation: operation, Status: status, Message: message, Paths: paths, DurationMS: time.Since(started).Milliseconds(), Error: errorMessage})
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

func (a *API) systemConfigPaths(w http.ResponseWriter, r *http.Request) {
	paths, err := config.ResolvePaths()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"dataDir":        paths.Dir,
		"defaultDataDir": paths.DefaultDir,
		"cloneRootDir":   paths.CloneRootDir,
	})
}

func (a *API) updateSystemConfigPaths(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DataDir string `json:"dataDir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	paths, err := config.SetDataDir(input.DataDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"dataDir":         paths.Dir,
		"defaultDataDir":  paths.DefaultDir,
		"cloneRootDir":    paths.CloneRootDir,
		"restartRequired": true,
	})
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

func respondWorkspaceFileResult(w http.ResponseWriter, data any, err error) {
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, data)
	case errors.Is(err, apperrors.ErrWorkspaceNotFound), errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, workspaceaccess.ErrHashRequired), errors.Is(err, workspaceaccess.ErrStaleContent):
		writeError(w, http.StatusConflict, workspaceaccess.ErrStaleContent.Error())
	case errors.Is(err, workspaceaccess.ErrDestinationExists):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func respondContentSearch(w http.ResponseWriter, data any, err error) {
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, data)
	case errors.Is(err, apperrors.ErrItemNotFound), errors.Is(err, apperrors.ErrWorkspaceNotFound), errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, context.Canceled):
		writeError(w, 499, "content search canceled")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func optionalBool(r *http.Request, name string) (bool, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return value, nil
}

func respondGitResult(w http.ResponseWriter, result models.GitOperationResult) {
	if result.Message == apperrors.ErrWorkspaceNotFound.Error() {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	writeJSON(w, statusForErrorFromResult(result), result)
}

func withRecoveryHint(result models.GitOperationResult) models.GitOperationResult {
	if !result.OK && result.RecoveryHint == "" {
		result.RecoveryHint = recoveryHint(result.Message)
	}
	return result
}

func statusForErrorFromResult(result models.GitOperationResult) int {
	if !result.OK {
		return http.StatusBadRequest
	}
	return http.StatusOK
}

func writeSSE(w http.ResponseWriter, event string, data any) {
	payload, _ := json.Marshal(data)
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(string(payload), "\n") {
		_, _ = fmt.Fprintf(w, "data: %s\n", line)
	}
	_, _ = fmt.Fprint(w, "\n")
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
	payload := map[string]string{"error": message}
	if hint := recoveryHint(message); hint != "" {
		payload["recoveryHint"] = hint
	}
	writeJSON(w, status, payload)
}

func recoveryHint(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "changed since it was loaded"):
		return "Reload the file to review the latest content, then apply your changes again."
	case strings.Contains(lower, "local changes"):
		return "Review local changes, then confirm the operation or commit them first."
	case strings.Contains(lower, "conflict"):
		return "Resolve or abort the current Git operation before continuing."
	case strings.Contains(lower, "outside configured sources"), strings.Contains(lower, "path escapes"):
		return "Choose a path inside a configured workspace source."
	default:
		return ""
	}
}

func Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s\n", r.Method, r.URL.Path)
	})
}
