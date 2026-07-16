package api

import (
	"context"
	"net/http"
)

func (a *API) healthPayload(ctx context.Context) (map[string]any, int) {
	payload := map[string]any{"ok": true}
	if a.databaseHealth == nil {
		return payload, http.StatusOK
	}
	database := a.databaseHealth.Health(ctx)
	payload["database"] = database
	if !database.OK {
		payload["ok"] = false
		return payload, http.StatusServiceUnavailable
	}
	return payload, http.StatusOK
}
