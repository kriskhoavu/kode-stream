package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
)

func (a *API) registerGinRoutes(api *gin.RouterGroup) {
	api.GET("/health", a.ginHealth)
	api.GET("/audit-events", a.ginAuditEvents)
}

func (a *API) ginHealth(c *gin.Context) {
	ginJSON(c, 200, map[string]any{"ok": true})
}

func (a *API) ginAuditEvents(c *gin.Context) {
	if a.audit == nil {
		ginJSON(c, 200, []models.AuditEvent{})
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	events, err := a.audit.Recent(limit * 2)
	if err != nil {
		ginAppError(c, apperrors.Infra(err.Error(), err))
		return
	}
	workspaceID := c.Query("workspaceId")
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
	ginJSON(c, 200, events)
}
