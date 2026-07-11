package navigation

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/httpx"
	"kode-stream/internal/common/models"
)

type ItemReader interface {
	Detail(string) (models.ItemDetail, error)
}

type NavigationController struct {
	repository *NavigationRepository
	items      ItemReader
}

func NewController(repository *NavigationRepository, items ItemReader) *NavigationController {
	return &NavigationController{repository: repository, items: items}
}

func (c *NavigationController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/saved-filters", c.filters)
	mux.HandleFunc("POST /api/saved-filters", c.saveFilter)
	mux.HandleFunc("DELETE /api/saved-filters/{id}", c.deleteFilter)
	mux.HandleFunc("GET /api/recent-items", c.recents)
	mux.HandleFunc("POST /api/recent-items", c.recordRecent)
}

func (c *NavigationController) filters(w http.ResponseWriter, _ *http.Request) {
	if c.repository == nil {
		httpx.WriteJSON(w, http.StatusOK, []models.SavedFilter{})
		return
	}
	filters, err := c.repository.Filters()
	c.respond(w, filters, err)
}

func (c *NavigationController) saveFilter(w http.ResponseWriter, r *http.Request) {
	if c.repository == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "saved filters are unavailable", nil)
		return
	}
	var filter models.SavedFilter
	if err := json.NewDecoder(r.Body).Decode(&filter); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body", nil)
		return
	}
	filter.Name = strings.TrimSpace(filter.Name)
	if filter.Name == "" {
		httpx.WriteError(w, http.StatusBadRequest, "saved filter name is required", nil)
		return
	}
	if !validAppRoute(filter.Route) {
		httpx.WriteError(w, http.StatusBadRequest, "saved filter route is invalid", nil)
		return
	}
	saved, err := c.repository.SaveFilter(filter)
	if err != nil {
		c.respond(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, saved)
}

func (c *NavigationController) deleteFilter(w http.ResponseWriter, r *http.Request) {
	if c.repository == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "saved filters are unavailable", nil)
		return
	}
	deleted, err := c.repository.DeleteFilter(r.PathValue("id"))
	if err != nil {
		c.respond(w, nil, err)
		return
	}
	if !deleted {
		httpx.WriteError(w, http.StatusNotFound, "saved filter not found", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (c *NavigationController) recents(w http.ResponseWriter, r *http.Request) {
	if c.repository == nil {
		httpx.WriteJSON(w, http.StatusOK, []models.RecentItem{})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	recents, err := c.repository.Recents(limit)
	c.respond(w, recents, err)
}

func (c *NavigationController) recordRecent(w http.ResponseWriter, r *http.Request) {
	if c.repository == nil || c.items == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "recent items are unavailable", nil)
		return
	}
	var input struct {
		ItemID string `json:"itemId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.ItemID) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "itemId is required", nil)
		return
	}
	item, err := c.items.Detail(input.ItemID)
	if errors.Is(err, apperrors.ErrItemNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "item not found", nil)
		return
	}
	if err != nil {
		c.respond(w, nil, err)
		return
	}
	recent := models.RecentItem{ItemID: item.ID, WorkspaceID: item.WorkspaceID, Title: item.Title, Subtitle: strings.Trim(strings.Join([]string{item.WorkspaceName, item.Identifier}, " · "), " ·"), Route: "/items/" + url.PathEscape(item.ID)}
	if err := c.repository.RecordRecent(recent); err != nil {
		c.respond(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (c *NavigationController) respond(w http.ResponseWriter, data any, err error) {
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, data)
}

func validAppRoute(route string) bool {
	return route == "/workstream" || route == "/workspaces" || route == "/settings" || route == "/knowledge" || strings.HasPrefix(route, "/items/") || strings.HasPrefix(route, "/workstream?") || strings.HasPrefix(route, "/knowledge?")
}
