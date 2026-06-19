package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apphealth "plan-manager/internal/application/health"
	appsearch "plan-manager/internal/application/search"
	"plan-manager/internal/audit"
	"plan-manager/internal/fileaccess"
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/itemindex"
	"plan-manager/internal/models"
	"plan-manager/internal/navigation"
	"plan-manager/internal/registry"
)

func TestFallbackItemPath(t *testing.T) {
	workspace := models.WorkspaceConfig{Sources: []string{"items"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{Scope: "api", Identifier: "DI-170"}}

	got := fallbackItemPath(workspace, item)
	if got != "items/api/DI-170" {
		t.Fatalf("fallbackItemPath() = %q", got)
	}
}

func TestFallbackItemPathRequiresPlanDirectory(t *testing.T) {
	item := models.ItemDetail{ItemSummary: models.ItemSummary{Scope: "api", Identifier: "DI-170"}}

	if got := fallbackItemPath(models.WorkspaceConfig{}, item); got != "" {
		t.Fatalf("fallbackItemPath() = %q, want empty", got)
	}
}

func TestFirstMarkdownParagraphReturnsFullParagraph(t *testing.T) {
	markdown := "# Title\n\nEvery controller repeats the same permission boilerplate: build an `actionList`, call `isInvalidOfferActions()`, return 403. Controllers also accept `@RequestParam OfferAction action` from the frontend, leaking authorization details into the client contract."

	got := firstMarkdownParagraph(markdown)
	if strings.Contains(got, "...") {
		t.Fatalf("paragraph was truncated: %q", got)
	}
	if !strings.Contains(got, "client contract") {
		t.Fatalf("paragraph did not include the full text: %q", got)
	}
}

func TestNormalizeItemDetailUsesEmptyCollections(t *testing.T) {
	item := normalizeItemDetail(models.ItemDetail{})
	if item.Tags == nil {
		t.Fatal("tags should be an empty slice, got nil")
	}
	if item.Documents == nil {
		t.Fatal("documents should be an empty slice, got nil")
	}
	if item.Metadata == nil {
		t.Fatal("metadata should be an empty map, got nil")
	}
}

func TestValidateGitPathsStaysInsideSources(t *testing.T) {
	workspace := models.WorkspaceConfig{Sources: []string{"items", "docs"}}
	if err := validateGitPaths(workspace, []string{"items/platform/PM-002/README.md", "docs/guide.md"}); err != nil {
		t.Fatalf("expected paths to be valid: %v", err)
	}
}

func TestValidateGitPathsRejectsEscapesAndUnregisteredPaths(t *testing.T) {
	workspace := models.WorkspaceConfig{Sources: []string{"items"}}
	for _, paths := range [][]string{
		{},
		{"../secret.md"},
		{"/tmp/secret.md"},
		{"src/main.go"},
	} {
		if err := validateGitPaths(workspace, paths); err == nil {
			t.Fatalf("expected %#v to be rejected", paths)
		}
	}
}

func TestRoutesListItemsPreservesJSONShape(t *testing.T) {
	dir := t.TempDir()
	idx := itemindex.New(filepath.Join(dir, "item-index.yaml"))
	updatedAt := time.Date(2026, 6, 20, 1, 2, 3, 0, time.UTC)
	if err := idx.ReplaceWorkspace("workspace-1", []models.ItemDetail{{
		ItemSummary: models.ItemSummary{
			ID:             "item-1",
			WorkspaceID:    "workspace-1",
			WorkspaceName:  "Workspace",
			Branch:         "main",
			Scope:          "platform",
			Identifier:     "PM-003",
			Title:          "Architecture",
			Status:         models.StatusDraft,
			UpdatedAt:      updatedAt,
			Description:    "Refactor architecture",
			MetadataSource: "item.yaml",
			ItemPath:       "plans/platform/PM-003",
		},
	}}, nil, updatedAt); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/items?workspaceId=workspace-1&q=architecture", nil)
	res := httptest.NewRecorder()
	New(nil, idx, nil, nil, nil, nil, nil).Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var items []models.ItemSummary
	if err := json.Unmarshal(res.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
	item := items[0]
	if item.ID != "item-1" || item.Identifier != "PM-003" || item.Status != models.StatusDraft || item.MetadataSource != "item.yaml" {
		t.Fatalf("unexpected item response: %+v", item)
	}
	if item.Tags == nil {
		t.Fatal("tags should be normalized to an empty array")
	}
}

func TestRoutesMissingItemReturnsNotFoundJSON(t *testing.T) {
	dir := t.TempDir()
	idx := itemindex.New(filepath.Join(dir, "item-index.yaml"))
	req := httptest.NewRequest(http.MethodGet, "/api/items/missing", nil)
	res := httptest.NewRecorder()

	New(nil, idx, nil, nil, nil, nil, nil).Routes().ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["error"] != "item not found" {
		t.Fatalf("error = %q", payload["error"])
	}
}

func TestReliabilityEndpointsReturnWorkspaceHealthAndRecentAuditEvents(t *testing.T) {
	apiHandler, workspace, _, auditStore := reliabilityTestAPI(t)
	if _, err := auditStore.Append(models.AuditEvent{WorkspaceID: workspace.ID, Operation: "scan", Status: models.AuditStatusSuccess, Message: "Scanned"}); err != nil {
		t.Fatal(err)
	}

	healthRequest := httptest.NewRequest(http.MethodGet, "/api/workspaces/"+workspace.ID+"/health", nil)
	healthResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(healthResponse, healthRequest)
	if healthResponse.Code != http.StatusOK {
		t.Fatalf("health status = %d, body = %s", healthResponse.Code, healthResponse.Body.String())
	}
	var workspaceHealth models.WorkspaceHealth
	if err := json.Unmarshal(healthResponse.Body.Bytes(), &workspaceHealth); err != nil {
		t.Fatal(err)
	}
	if workspaceHealth.WorkspaceID != workspace.ID || workspaceHealth.Summary != models.HealthStatusOK {
		t.Fatalf("health = %#v", workspaceHealth)
	}

	auditRequest := httptest.NewRequest(http.MethodGet, "/api/audit-events?workspaceId="+workspace.ID+"&limit=1", nil)
	auditResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(auditResponse, auditRequest)
	var events []models.AuditEvent
	if err := json.Unmarshal(auditResponse.Body.Bytes(), &events); err != nil {
		t.Fatal(err)
	}
	if auditResponse.Code != http.StatusOK || len(events) != 1 || events[0].Operation != "scan" {
		t.Fatalf("audit status = %d, events = %#v", auditResponse.Code, events)
	}
}

func TestSaveFileStaleHashReturnsRecoveryHintAndAuditEvent(t *testing.T) {
	apiHandler, workspace, idx, auditStore := reliabilityTestAPI(t)
	itemPath := "plans/platform/PM-004"
	if err := os.MkdirAll(filepath.Join(workspace.Path, itemPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace.Path, itemPath, "README.md"), []byte("# Current\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := idx.ReplaceWorkspace(workspace.ID, []models.ItemDetail{{ItemSummary: models.ItemSummary{ID: "item-1", WorkspaceID: workspace.ID, ItemPath: itemPath, Title: "PM-004", Identifier: "PM-004", Scope: "platform"}}}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	body := strings.NewReader(`{"content":"# Changed\n","expectedHash":"stale"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/items/item-1/files/README_md", body)
	response := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(response, request)

	var payload map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadRequest || payload["recoveryHint"] == "" {
		t.Fatalf("status = %d, payload = %#v", response.Code, payload)
	}
	events, err := auditStore.Recent(1)
	if err != nil || len(events) != 1 || events[0].Status != models.AuditStatusBlocked {
		t.Fatalf("events = %#v, err = %v", events, err)
	}
}

func TestGitPullDirtyTreeReturnsRecoveryHint(t *testing.T) {
	apiHandler, workspace, _, _ := reliabilityTestAPI(t)
	if err := os.WriteFile(filepath.Join(workspace.Path, "plans", "dirty.md"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/git/pull", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(response, request)

	var payload models.GitOperationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadRequest || payload.OK || payload.RecoveryHint == "" {
		t.Fatalf("status = %d, payload = %#v", response.Code, payload)
	}
}

func TestGitCommitRejectsPathOutsideConfiguredSources(t *testing.T) {
	apiHandler, workspace, _, _ := reliabilityTestAPI(t)
	body := strings.NewReader(`{"message":"test","paths":["../secret.md"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/git/commit", body)
	response := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(response, request)

	var payload models.GitOperationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadRequest || payload.OK {
		t.Fatalf("status = %d, payload = %#v", response.Code, payload)
	}
}

func TestSearchEndpointSupportsAllAndWorkspaceScopedQueries(t *testing.T) {
	apiHandler, workspace, idx, _ := reliabilityTestAPI(t)
	if err := idx.ReplaceWorkspace(workspace.ID, []models.ItemDetail{{ItemSummary: models.ItemSummary{ID: "one", WorkspaceID: workspace.ID, Identifier: "PM-005", Title: "Search"}}}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := idx.ReplaceWorkspace("other", []models.ItemDetail{{ItemSummary: models.ItemSummary{ID: "two", WorkspaceID: "other", Identifier: "PM-005", Title: "Other search"}}}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	apiHandler.search = appsearch.New(idx)

	for _, test := range []struct {
		path string
		want int
	}{{"/api/search?q=PM-005", 2}, {"/api/search?q=PM-005&workspaceId=" + workspace.ID, 1}} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		apiHandler.Routes().ServeHTTP(response, request)
		var results []models.SearchResult
		if err := json.Unmarshal(response.Body.Bytes(), &results); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || len(results) != test.want {
			t.Fatalf("GET %s status=%d results=%#v", test.path, response.Code, results)
		}
	}
}

func TestSavedFilterEndpointsValidateCreateListAndDelete(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	dir := t.TempDir()
	apiHandler.navigation = navigation.New(filepath.Join(dir, "filters.yaml"), filepath.Join(dir, "recents.yaml"))

	invalid := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(invalid, httptest.NewRequest(http.MethodPost, "/api/saved-filters", strings.NewReader(`{"name":"","route":"https://example.com"}`)))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d", invalid.Code)
	}

	createdResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(createdResponse, httptest.NewRequest(http.MethodPost, "/api/saved-filters", strings.NewReader(`{"name":"Drafts","route":"/kanban","filters":{"statuses":["draft"]}}`)))
	var created models.SavedFilter
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("created = %#v, err=%v", created, err)
	}
	listResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/saved-filters", nil))
	var filters []models.SavedFilter
	if err := json.Unmarshal(listResponse.Body.Bytes(), &filters); err != nil || len(filters) != 1 {
		t.Fatalf("filters = %#v, err=%v", filters, err)
	}
	deleteResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(deleteResponse, httptest.NewRequest(http.MethodDelete, "/api/saved-filters/"+created.ID, nil))
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete status = %d", deleteResponse.Code)
	}
}

func TestRecentItemEndpointOrdersLatestOpenFirst(t *testing.T) {
	apiHandler, workspace, idx, _ := reliabilityTestAPI(t)
	dir := t.TempDir()
	apiHandler.navigation = navigation.New(filepath.Join(dir, "filters.yaml"), filepath.Join(dir, "recents.yaml"))
	items := []models.ItemDetail{
		{ItemSummary: models.ItemSummary{ID: "one", WorkspaceID: workspace.ID, WorkspaceName: workspace.Name, Identifier: "PM-001", Title: "One", ItemPath: "plans/one"}},
		{ItemSummary: models.ItemSummary{ID: "two", WorkspaceID: workspace.ID, WorkspaceName: workspace.Name, Identifier: "PM-002", Title: "Two", ItemPath: "plans/two"}},
	}
	if err := idx.ReplaceWorkspace(workspace.ID, items, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two", "one"} {
		response := httptest.NewRecorder()
		apiHandler.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/recent-items", strings.NewReader(`{"itemId":"`+id+`"}`)))
		if response.Code != http.StatusOK {
			t.Fatalf("record %s status=%d body=%s", id, response.Code, response.Body.String())
		}
		time.Sleep(time.Millisecond)
	}
	response := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/recent-items", nil))
	var recents []models.RecentItem
	if err := json.Unmarshal(response.Body.Bytes(), &recents); err != nil || len(recents) != 2 || recents[0].ItemID != "one" {
		t.Fatalf("recents = %#v, err=%v", recents, err)
	}
}

func reliabilityTestAPI(t *testing.T) (*API, models.WorkspaceConfig, *itemindex.Index, *audit.Store) {
	t.Helper()
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	commit := exec.Command("git", "-C", root, "commit", "--allow-empty", "-m", "init")
	commit.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	if err := os.Mkdir(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := gitadapter.New()
	reg := registry.New(filepath.Join(t.TempDir(), "workspaces.yaml"), git)
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Test", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	idx := itemindex.New(filepath.Join(t.TempDir(), "item-index.yaml"))
	if err := idx.ReplaceWorkspace(workspace.ID, nil, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := reg.TouchScanned(workspace.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	workspace, _, _ = reg.Get(workspace.ID)
	auditStore := audit.New(filepath.Join(t.TempDir(), "audit-log.jsonl"))
	healthService := apphealth.New(reg, idx, git)
	return NewWithReliability(reg, idx, nil, fileaccess.New(), nil, git, nil, auditStore, healthService), workspace, idx, auditStore
}
