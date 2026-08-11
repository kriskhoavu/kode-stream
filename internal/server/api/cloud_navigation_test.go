package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/navigation"
)

func TestCloudNavigationHTTPIsolatesTwoOwnersAndDoesNotDiscloseDeletes(t *testing.T) {
	apiHandler, workspace, index, _ := reliabilityTestAPI(t)
	apiHandler = withTestRuntime(apiHandler, testAppOIDCRuntimeConfig())
	store := navigation.New(filepath.Join(t.TempDir(), "filters.yaml"), filepath.Join(t.TempDir(), "recents.yaml"))
	legacy, err := store.SaveFilter(models.SavedFilter{Name: "Legacy local", Route: "/workstream"})
	if err != nil {
		t.Fatal(err)
	}
	apiHandler.navigation = navigation.NewController(store, apiHandler.item.items, func(ctx context.Context) string {
		if session, ok := cloudSessionFromContext(ctx); ok {
			return session.User.ID
		}
		return navigation.LocalOwner
	})
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "item", WorkspaceID: workspace.ID, WorkspaceName: workspace.Name, Title: "Plan", Identifier: "PM-1", UpdatedAt: time.Now().UTC()}}
	if err := index.ReplaceWorkspace(workspace.ID, []models.ItemDetail{item}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	handler := apiHandler.Routes()

	create := cloudNavigationRequest(handler, http.MethodPost, "/api/saved-filters", `{"name":"Private","route":"/workstream"}`, "owner-a")
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	var filter models.SavedFilter
	if err := json.Unmarshal(create.Body.Bytes(), &filter); err != nil {
		t.Fatal(err)
	}
	if response := cloudNavigationRequest(handler, http.MethodPost, "/api/recent-items", `{"itemId":"item"}`, "owner-a"); response.Code != http.StatusOK {
		t.Fatalf("recent status=%d body=%s", response.Code, response.Body.String())
	}

	for _, path := range []string{"/api/saved-filters", "/api/recent-items"} {
		response := cloudNavigationRequest(handler, http.MethodGet, path, "", "owner-b")
		if response.Code != http.StatusOK || response.Body.String() != "[]\n" {
			t.Fatalf("owner-b %s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	deleted := cloudNavigationRequest(handler, http.MethodDelete, "/api/saved-filters/"+filter.ID, "", "owner-b")
	if deleted.Code != http.StatusNotFound {
		t.Fatalf("cross-owner delete status=%d body=%s", deleted.Code, deleted.Body.String())
	}
	ownerA := cloudNavigationRequest(handler, http.MethodGet, "/api/saved-filters", "", "owner-a")
	if ownerA.Code != http.StatusOK || !strings.Contains(ownerA.Body.String(), filter.ID) || strings.Contains(ownerA.Body.String(), legacy.ID) {
		t.Fatalf("owner-a filter disappeared: %d %s", ownerA.Code, ownerA.Body.String())
	}
	localFilters, err := store.FiltersForOwner(navigation.LocalOwner)
	if err != nil || len(localFilters) != 1 || localFilters[0].ID != legacy.ID {
		t.Fatalf("legacy Local quarantine=%#v err=%v", localFilters, err)
	}
}

func cloudNavigationRequest(handler http.Handler, method, path, body, subject string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("X-Kode-Stream-Subject", subject)
	request.Header.Set("X-Kode-Stream-Role", "editor")
	if isMutatingMethod(method) {
		request.Header.Set(csrfHeader, stableCloudUserID(subject+":csrf"))
	}
	handler.ServeHTTP(response, request)
	return response
}
