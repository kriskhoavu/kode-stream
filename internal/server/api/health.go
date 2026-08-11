package api

import (
	"context"
	"net/http"
)

func (a *healthController) healthPayload(ctx context.Context) (map[string]any, int) {
	payload := map[string]any{"ok": true}
	if a.database == nil {
		return payload, http.StatusOK
	}
	database := a.database.Health(ctx)
	payload["database"] = database
	if !database.OK {
		payload["ok"] = false
		return payload, http.StatusServiceUnavailable
	}
	return payload, http.StatusOK
}
