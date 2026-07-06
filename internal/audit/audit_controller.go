package audit

import (
	"net/http"
	"strconv"

	"plan-manager/internal/common/httpx"
	"plan-manager/internal/common/models"
)

type AuditController struct{ repository *AuditRepository }

func NewController(repository *AuditRepository) *AuditController {
	return &AuditController{repository: repository}
}

func (c *AuditController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/audit-events", c.events)
}

func (c *AuditController) events(w http.ResponseWriter, r *http.Request) {
	if c.repository == nil {
		httpx.WriteJSON(w, http.StatusOK, []models.AuditEvent{})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	events, err := c.repository.Recent(limit * 2)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), nil)
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
	httpx.WriteJSON(w, http.StatusOK, events)
}
