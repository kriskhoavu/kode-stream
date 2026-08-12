package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kode-stream/internal/audit"
	"kode-stream/internal/common/models"
	appsearch "kode-stream/internal/search"
	"kode-stream/internal/system"
	appworkspace "kode-stream/internal/workspace"
)

type auditRecorder struct {
	repository audit.Repository
	reader     auditEventReader
}

func (a auditRecorder) recordOwned(ownerUserID, actorUserID, workspaceID, operation, message string) {
	if a.repository == nil {
		return
	}
	_, _ = a.repository.Append(models.AuditEvent{OwnerUserID: ownerUserID, ActorUserID: actorUserID, WorkspaceID: workspaceID, Operation: operation, Status: models.AuditStatusSuccess, Message: message, Time: time.Now().UTC(), Paths: []string{}})
}

type auditEventReader interface {
	RecentContext(context.Context, int) ([]models.AuditEvent, error)
	QueryContext(context.Context, audit.Query) ([]models.AuditEvent, error)
}

type stateController struct {
	workspaces    *appworkspace.Service
	runtimeConfig system.RuntimeConfig
}

type searchController struct{ search *appsearch.SearchService }

type auditController struct{ reader auditEventReader }

type healthController struct{ database databaseHealthChecker }

func (a *healthController) health(w http.ResponseWriter, r *http.Request) {
	payload, status := a.healthPayload(r.Context())
	writeJSON(w, status, payload)
}

func (a *stateController) state(w http.ResponseWriter, r *http.Request) {
	state, err := a.workspaces.State()
	state.Mode = a.runtimeConfig.Mode
	state.User = a.runtimeConfig.User
	state.Role = a.runtimeConfig.Role
	state.Capabilities = a.runtimeConfig.Capabilities
	state.Agent = a.runtimeConfig.Agent
	if session, ok := cloudSessionFromContext(r.Context()); ok {
		state.User = &session.User
		state.Role = session.User.Role
		state.Capabilities = roleCapabilities(session.User.Role)
	}
	respond(w, state, err)
}

func (a *auditController) auditEvents(w http.ResponseWriter, r *http.Request) {
	if a.reader == nil {
		writeJSON(w, http.StatusOK, []models.AuditEvent{})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := audit.Query{WorkspaceID: r.URL.Query().Get("workspaceId"), Limit: limit}
	if session, ok := cloudSessionFromContext(r.Context()); ok {
		query.OwnerUserID = session.User.ID
	}
	events, err := a.reader.QueryContext(r.Context(), query)
	if err != nil {
		respond(w, nil, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (a *searchController) searchItems(w http.ResponseWriter, r *http.Request) {
	if a.search == nil {
		writeJSON(w, http.StatusOK, []models.SearchResult{})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	types := strings.Split(strings.TrimSpace(r.URL.Query().Get("types")), ",")
	if len(types) == 1 && types[0] == "" {
		types = nil
	}
	results, err := a.search.SearchContext(r.Context(), models.SearchQuery{Text: r.URL.Query().Get("q"), WorkspaceID: r.URL.Query().Get("workspaceId"), Types: types, Limit: limit})
	respondSearch(w, results, err)
}
