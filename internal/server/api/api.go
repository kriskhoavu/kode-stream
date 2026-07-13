package api

// Package api provides the Server HTTP transport.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	appaisession "kode-stream/internal/ai"
	"kode-stream/internal/audit"
	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/httpx"
	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/content"
	appgit "kode-stream/internal/git"
	appitem "kode-stream/internal/item"
	"kode-stream/internal/item/index"
	"kode-stream/internal/item/writer"
	appjira "kode-stream/internal/jira"
	knowledgeindex "kode-stream/internal/knowledge"
	"kode-stream/internal/navigation"
	appruntime "kode-stream/internal/runtime"
	appsearch "kode-stream/internal/search"
	"kode-stream/internal/system"
	appverification "kode-stream/internal/verification"
	appworkspace "kode-stream/internal/workspace"
	workspacehealth "kode-stream/internal/workspace"
	workspaceaccess "kode-stream/internal/workspace/files"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
	appworkstream "kode-stream/internal/workstream"
)

type API struct {
	workspaces     *appworkspace.Service
	workstream     *appworkstream.Service
	items          *appitem.Service
	gitOps         *appgit.Service
	dialog         *system.Dialog
	audit          *audit.Store
	auditReader    auditEventReader
	healthService  *workspacehealth.HealthService
	search         *appsearch.SearchService
	navigation     *navigation.Store
	workspaceFiles *appworkspace.WorkspaceFileService
	contentSearch  *appsearch.ContentSearchService
	aiSessions     *appaisession.Service
	jira           *appjira.Service
	knowledge      *knowledgeindex.KnowledgeService
	verification   *appverification.Service
}

func (a *API) WithJira(service *appjira.Service) *API {
	a.jira = service
	return a
}

func (a *API) WithAISessions(service *appaisession.Service) *API {
	a.aiSessions = service
	return a
}

func (a *API) WithKnowledge(service *knowledgeindex.KnowledgeService) *API {
	a.knowledge = service
	return a
}

func (a *API) WithVerification(service *appverification.Service) *API {
	a.verification = service
	return a
}

func New(reg *registry.Registry, idx *itemindex.Index, scan *scanner.Scanner, files *fileaccess.Access, writer *itemwriter.Writer, git *appgit.GitAdapter, dialog *system.Dialog) *API {
	return NewWithReliability(reg, idx, scan, files, writer, git, dialog, nil, nil)
}

func NewWithReliability(reg *registry.Registry, idx *itemindex.Index, scan *scanner.Scanner, files *fileaccess.Access, writer *itemwriter.Writer, git *appgit.GitAdapter, dialog *system.Dialog, auditStore *audit.Store, healthService *workspacehealth.HealthService) *API {
	return NewWithServices(reg, idx, scan, files, writer, git, dialog, auditStore, healthService, nil, nil)
}

func NewWithServices(reg *registry.Registry, idx *itemindex.Index, scan *scanner.Scanner, files *fileaccess.Access, writer *itemwriter.Writer, git *appgit.GitAdapter, dialog *system.Dialog, auditStore *audit.Store, healthService *workspacehealth.HealthService, searchService *appsearch.SearchService, navigationStore *navigation.Store) *API {
	var refresher appworkspace.Refresher
	if writer != nil {
		refresher = writer
	}
	var auditReader auditEventReader
	if auditStore != nil {
		auditReader = audit.NewCachedEventReader(auditStore, 2*time.Second, time.Now)
	}
	workspaceFileAccess := workspaceaccess.New()
	workspaceService := appworkspace.New(reg, idx, scan, writer, git)
	runtimeService := appruntime.NewService()
	if auditStore != nil {
		workspaceService.ConfigureAudit(auditStore)
	}
	return &API{
		workspaces:     workspaceService,
		workstream:     appworkstream.New(reg, idx, scan, git),
		items:          appitem.New(reg, idx, files, writer, git),
		gitOps:         appgit.NewService(reg, writer, git),
		dialog:         dialog,
		audit:          auditStore,
		auditReader:    auditReader,
		healthService:  healthService,
		search:         searchService,
		navigation:     navigationStore,
		workspaceFiles: appworkspace.NewWorkspaceFileService(reg, workspaceFileAccess, git, auditStore, refresher),
		contentSearch:  appsearch.NewContentSearchService(reg, idx, workspaceFileAccess),
		verification:   appverification.NewService(reg, runtimeService),
	}
}

func (a *API) Routes() http.Handler {
	return newTransport(a.registerGinRoutes)
}

func (a *API) previewWorkspaceImport(w http.ResponseWriter, r *http.Request) {
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

func (a *API) importWorkspaces(w http.ResponseWriter, r *http.Request) {
	var input models.WorkspaceImportRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	results, err := a.workspaces.Import(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (a *API) startEmbeddedAISession(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "embedded AI sessions are unavailable")
		return
	}
	var input appaisession.EmbeddedInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.aiSessions.StartEmbedded(r.PathValue("id"), input)
	if err == nil {
		writeJSON(w, http.StatusCreated, result)
		return
	}
	var launchErr *appaisession.LaunchError
	if !errors.As(err, &launchErr) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := http.StatusBadRequest
	if launchErr.Code == "item_not_found" || launchErr.Code == "workspace_not_found" {
		status = http.StatusNotFound
	}
	if launchErr.Code == "launch_failed" {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]string{"error": launchErr.Error(), "code": launchErr.Code})
}

func (a *API) embeddedAISession(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil || a.aiSessions.EmbeddedManager() == nil {
		writeError(w, http.StatusServiceUnavailable, "embedded AI sessions are unavailable")
		return
	}
	session, err := a.aiSessions.EmbeddedManager().Get(r.PathValue("sessionId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (a *API) cancelEmbeddedAISession(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil || a.aiSessions.EmbeddedManager() == nil {
		writeError(w, http.StatusServiceUnavailable, "embedded AI sessions are unavailable")
		return
	}
	session, err := a.aiSessions.EmbeddedManager().Cancel(r.PathValue("sessionId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

type channelInput struct {
	Type    string `json:"type"`
	Data    string `json:"data,omitempty"`
	Columns uint16 `json:"columns,omitempty"`
	Rows    uint16 `json:"rows,omitempty"`
}
type channelOutput struct {
	Type     string `json:"type"`
	Data     string `json:"data,omitempty"`
	Encoding string `json:"encoding,omitempty"`
	State    string `json:"state,omitempty"`
	ExitCode *int   `json:"exitCode,omitempty"`
	Message  string `json:"message,omitempty"`
}

func (a *API) embeddedAISessionChannel(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil || a.aiSessions.EmbeddedManager() == nil {
		writeError(w, http.StatusServiceUnavailable, "embedded AI sessions are unavailable")
		return
	}
	manager := a.aiSessions.EmbeddedManager()
	id := r.PathValue("sessionId")
	if err := manager.Authenticate(id, r.URL.Query().Get("token")); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid session grant")
		return
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(request *http.Request) bool {
		return request.Header.Get("Origin") == "http://"+request.Host || request.Header.Get("Origin") == "https://"+request.Host
	}}
	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	output, buffered, unsubscribe, err := manager.Subscribe(id)
	if err != nil {
		return
	}
	defer unsubscribe()
	if len(buffered) > 0 {
		_ = connection.WriteJSON(channelOutput{Type: "output", Data: base64.StdEncoding.EncodeToString(buffered), Encoding: "base64"})
	}
	state, _ := manager.Get(id)
	_ = connection.WriteJSON(channelOutput{Type: "state", State: state.State, ExitCode: state.ExitCode})
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			var message channelInput
			if err := connection.ReadJSON(&message); err != nil {
				return
			}
			switch message.Type {
			case "input":
				decoded, err := base64.StdEncoding.DecodeString(message.Data)
				if err == nil {
					_ = manager.Write(id, decoded)
				}
			case "resize":
				_ = manager.Resize(id, message.Columns, message.Rows)
			case "cancel":
				_, _ = manager.Cancel(id)
			case "heartbeat":
			}
		}
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	lastState := state.State
	for {
		select {
		case data, ok := <-output:
			if !ok {
				return
			}
			if err := connection.WriteJSON(channelOutput{Type: "output", Data: base64.StdEncoding.EncodeToString(data), Encoding: "base64"}); err != nil {
				return
			}
		case <-ticker.C:
			current, err := manager.Get(id)
			if err != nil {
				return
			}
			if current.State != lastState {
				if err := connection.WriteJSON(channelOutput{Type: "state", State: current.State, ExitCode: current.ExitCode}); err != nil {
					return
				}
				lastState = current.State
				if current.State != "running" && current.State != "starting" {
					return
				}
			}
		case <-done:
			return
		}
	}
}

func (a *API) aiSessionEligibility(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session launch is unavailable")
		return
	}
	result, err := a.aiSessions.Eligibility(r.PathValue("id"))
	if err == nil {
		writeJSON(w, http.StatusOK, result)
		return
	}
	var launchErr *appaisession.LaunchError
	if errors.As(err, &launchErr) && (launchErr.Code == "item_not_found" || launchErr.Code == "workspace_not_found") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": launchErr.Error(), "code": launchErr.Code})
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func (a *API) launchAISession(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session launch is unavailable")
		return
	}
	var input appaisession.LaunchInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.aiSessions.Launch(r.PathValue("id"), input)
	if err == nil {
		writeJSON(w, http.StatusAccepted, result)
		return
	}
	var launchErr *appaisession.LaunchError
	if !errors.As(err, &launchErr) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := http.StatusBadRequest
	if launchErr.Code == "item_not_found" || launchErr.Code == "workspace_not_found" {
		status = http.StatusNotFound
	} else if launchErr.Code == "launch_failed" {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]string{"error": launchErr.Error(), "code": launchErr.Code})
}

func (a *API) aiCapabilities(w http.ResponseWriter, _ *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session settings are unavailable")
		return
	}
	capabilities, err := a.aiSessions.Capabilities()
	respond(w, capabilities, err)
}

func (a *API) aiPresets(w http.ResponseWriter, _ *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session settings are unavailable")
		return
	}
	writeJSON(w, http.StatusOK, a.aiSessions.Presets())
}

func (a *API) aiProviderCapabilities(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session settings are unavailable")
		return
	}
	result, err := a.aiSessions.ProviderCapabilities(r.PathValue("id"), r.URL.Query().Get("itemId"))
	if err == nil {
		writeJSON(w, http.StatusOK, result)
		return
	}
	var launchErr *appaisession.LaunchError
	if errors.As(err, &launchErr) && (launchErr.Code == "ai_provider_missing" || launchErr.Code == "item_not_found" || launchErr.Code == "workspace_not_found") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": launchErr.Error(), "code": launchErr.Code})
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func (a *API) aiSettings(w http.ResponseWriter, _ *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session settings are unavailable")
		return
	}
	settings, err := a.aiSessions.Settings()
	respond(w, settings, err)
}

func (a *API) saveAISettings(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session settings are unavailable")
		return
	}
	var settings appaisession.Settings
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	saved, err := a.aiSessions.Save(settings)
	respond(w, saved, err)
}

func (a *API) jiraAttachment(w http.ResponseWriter, r *http.Request) {
	if a.jira == nil {
		writeError(w, http.StatusServiceUnavailable, "Jira integration is unavailable")
		return
	}
	content, err := a.jira.Attachment(r.Context(), r.PathValue("id"), r.PathValue("attachmentId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filename := sanitizeDownloadName(content.Filename)
	disposition := "attachment"
	if safeInlineMediaType(content.MediaType) {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", content.MediaType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename=%q`, disposition, filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(content.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content.Data)
}

func safeInlineMediaType(value string) bool {
	switch strings.ToLower(value) {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}
func sanitizeDownloadName(value string) string {
	value = filepath.Base(strings.ReplaceAll(value, "\\", "/"))
	value = strings.Map(func(r rune) rune {
		if r < ' ' || r == 127 || r == '"' {
			return -1
		}
		return r
	}, value)
	if strings.TrimSpace(value) == "" || value == "." {
		return "attachment"
	}
	return value
}

func (a *API) jiraIssue(w http.ResponseWriter, r *http.Request) { a.respondJiraIssue(w, r, false) }
func (a *API) refreshJiraIssue(w http.ResponseWriter, r *http.Request) {
	a.respondJiraIssue(w, r, true)
}

func (a *API) workspaceJiraIssue(w http.ResponseWriter, r *http.Request) {
	if a.jira == nil {
		writeError(w, http.StatusServiceUnavailable, "Jira integration is unavailable")
		return
	}
	result, err := a.jira.WorkspaceIssue(r.Context(), r.PathValue("id"), r.PathValue("issueKey"), false)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) respondJiraIssue(w http.ResponseWriter, r *http.Request, refresh bool) {
	if a.jira == nil {
		writeError(w, http.StatusServiceUnavailable, "Jira integration is unavailable")
		return
	}
	result, err := a.jira.Issue(r.Context(), r.PathValue("id"), refresh)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) testJiraConnection(w http.ResponseWriter, r *http.Request) {
	if a.jira == nil {
		writeError(w, http.StatusServiceUnavailable, "Jira integration is unavailable")
		return
	}
	var connection models.JiraConnection
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&connection); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.jira.TestConnection(r.Context(), r.PathValue("id"), &connection)
	respond(w, result, err)
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

func (a *API) knowledgeWikis(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	wikis, err := a.knowledge.Wikis(r.URL.Query().Get("workspaceId"))
	a.respondKnowledge(w, wikis, err)
}

func (a *API) knowledgePages(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	pages, warnings, err := a.knowledge.Pages(r.PathValue("workspaceID"), r.PathValue("root"))
	a.respondKnowledge(w, map[string]any{"pages": pages, "warnings": warnings}, err)
}

func (a *API) knowledgePage(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	page, err := a.knowledge.Page(r.PathValue("workspaceID"), r.PathValue("root"), r.PathValue("slug"))
	a.respondKnowledge(w, page, err)
}

func (a *API) knowledgeGraph(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	graph, err := a.knowledge.Graph(r.PathValue("workspaceID"), r.PathValue("root"))
	a.respondKnowledge(w, graph, err)
}

func (a *API) knowledgeRescan(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	result, err := a.knowledge.Rescan(r.Context(), r.PathValue("workspaceID"), r.PathValue("root"))
	a.respondKnowledgeAction(w, result, err)
}

func (a *API) knowledgeSync(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	var input models.GitOperationInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.knowledge.Sync(r.Context(), r.PathValue("workspaceID"), input)
	a.respondKnowledgeAction(w, result, err)
}

func (a *API) knowledgeEnrich(w http.ResponseWriter, r *http.Request) {
	if a.knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge is unavailable")
		return
	}
	var input models.GitOperationInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.knowledge.Enrich(r.Context(), r.PathValue("workspaceID"), input.Confirm)
	a.respondKnowledgeAction(w, result, err)
}

func (a *API) respondKnowledgeAction(w http.ResponseWriter, result knowledgeindex.KnowledgeActionResult, err error) {
	switch {
	case errors.Is(err, knowledgeindex.ErrConfirmationRequired), errors.Is(err, knowledgeindex.ErrEnrichNotConfigured), errors.Is(err, knowledgeindex.ErrKnowledgeDisabled):
		writeError(w, http.StatusConflict, err.Error())
	case err != nil:
		a.respondKnowledge(w, result, err)
	case !result.OK:
		writeJSON(w, http.StatusUnprocessableEntity, result)
	default:
		writeJSON(w, http.StatusOK, result)
	}
}

func (a *API) respondKnowledge(w http.ResponseWriter, data any, err error) {
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, data)
	case errors.Is(err, knowledgeindex.ErrWorkspaceNotFound), errors.Is(err, knowledgeindex.ErrWikiNotFound), errors.Is(err, knowledgeindex.ErrPageNotFound), errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, knowledgeindex.ErrUnsafePath), errors.Is(err, fileaccess.ErrUnsupportedContent):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, knowledgeindex.ErrKnowledgeDisabled):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "knowledge request failed")
	}
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

func (a *API) loadWorkstreamBranch(w http.ResponseWriter, r *http.Request) {
	var input models.WorkstreamBranchLoadInput
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	started := time.Now()
	result, err := a.workstream.LoadBranch(r.PathValue("id"), input)
	a.record(r.PathValue("id"), "", "workstream_branch_load", "Workstream branch loaded.", nil, started, err)
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
	result, err := a.workspaceFiles.CreateFile(r.PathValue("id"), input)
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

func (a *API) itemVerificationTests(w http.ResponseWriter, r *http.Request) {
	tests, err := a.items.VerificationTests(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	respond(w, tests, err)
}

func (a *API) saveItemVerificationTests(w http.ResponseWriter, r *http.Request) {
	var input models.VerificationTestSelection
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	tests, err := a.items.SaveVerificationTests(r.PathValue("id"), input)
	if errors.Is(err, apperrors.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	respond(w, tests, err)
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
	if _, err := a.audit.Append(models.AuditEvent{WorkspaceID: workspaceID, ItemID: itemID, Operation: operation, Status: status, Message: message, Paths: paths, DurationMS: time.Since(started).Milliseconds(), Error: errorMessage}); err == nil {
		if invalidator, ok := a.auditReader.(interface{ Invalidate() }); ok {
			invalidator.Invalidate()
		}
	}
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
	paths, err := system.ResolvePaths()
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
	paths, err := system.SetDataDir(input.DataDir)
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
	httpx.WriteJSON(w, status, data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	httpx.WriteError(w, status, message, recoveryHint)
}

func recoveryHint(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "jira token environment variable"):
		return "Set the configured environment variable or add it to ~/.creds.zsh or ~/.creds.sh, then restart Kode Stream."
	case strings.Contains(lower, "jira authentication"):
		return "Check the Jira account and token configured for this workspace."
	case strings.Contains(lower, "jira project"):
		return "Check the Jira project key and account permissions."
	case strings.Contains(lower, "jira is unavailable"):
		return "Check the Jira base URL, network access, and server availability."
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
