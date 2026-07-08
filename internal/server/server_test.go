package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSPAHandlerRejectsDeprecatedSurfaceRoutes(t *testing.T) {
	handler := spaHandler()

	for _, path := range []string{"/kanban", "/kanban/", "/kanban/items/PM-025", "/workbench", "/workbench/items/PM-025", "/workspace", "/workspace/items/PM-025"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))

		if response.Code != http.StatusNotFound {
			t.Fatalf("expected %s to return 404, got %d", path, response.Code)
		}
	}
}

func TestSPAHandlerServesWorkstreamRoute(t *testing.T) {
	response := httptest.NewRecorder()

	spaHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/workstream", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected workstream route to return 200, got %d", response.Code)
	}
}
