package workspace

import (
	"net/http"

	"kode-stream/internal/common/httpx"
)

type HealthController struct{}

func NewHealthController() *HealthController { return &HealthController{} }

func (c *HealthController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", c.health)
}

func (c *HealthController) health(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}
