package api

// Package api provides the Server HTTP transport.

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appaisession "kode-stream/internal/ai"
	"kode-stream/internal/audit"
	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/content"
	gitadapter "kode-stream/internal/git"
	"kode-stream/internal/item/index"
	"kode-stream/internal/item/writer"
	"kode-stream/internal/navigation"
	appsearch "kode-stream/internal/search"
	workspacehealth "kode-stream/internal/workspace"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
)

func TestAISettingsRoutesReadValidateAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai-settings.yaml")
	handler := New(nil, nil, nil, nil, nil, nil, nil).WithAISessions(appaisession.New(appaisession.NewSettingsRepository(path))).Routes()

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/ai/settings", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", get.Code, get.Body.String())
	}
	var defaults appaisession.Settings
	if err := json.Unmarshal(get.Body.Bytes(), &defaults); err != nil {
		t.Fatal(err)
	}
	if len(defaults.Providers) != 4 || defaults.DefaultProvider == "" {
		t.Fatalf("defaults = %#v", defaults)
	}

	defaults.DefaultProvider = "claude"
	body, err := json.Marshal(defaults)
	if err != nil {
		t.Fatal(err)
	}
	put := httptest.NewRecorder()
	handler.ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/api/ai/settings", strings.NewReader(string(body))))
	if put.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", put.Code, put.Body.String())
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "defaultProvider: claude") {
		t.Fatalf("saved data = %q, err = %v", data, err)
	}

	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodPut, "/api/ai/settings", strings.NewReader(`{"defaultProvider":"missing"}`)))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, body = %s", invalid.Code, invalid.Body.String())
	}
}

func TestAICapabilitiesRouteReturnsStableShape(t *testing.T) {
	service := appaisession.New(appaisession.NewSettingsRepository(filepath.Join(t.TempDir(), "ai-settings.yaml")))
	handler := New(nil, nil, nil, nil, nil, nil, nil).WithAISessions(service).Routes()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/ai/capabilities", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var capabilities []appaisession.Capability
	if err := json.Unmarshal(response.Body.Bytes(), &capabilities); err != nil {
		t.Fatal(err)
	}
	if len(capabilities) < 4 {
		t.Fatalf("capabilities = %#v", capabilities)
	}
	for _, capability := range capabilities {
		if capability.ID == "" || capability.Kind == "" || capability.Executable == "" {
			t.Fatalf("invalid capability = %#v", capability)
		}
	}
}

func TestAIProviderCapabilitiesRouteReturnsCatalog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".codex", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "skills", "implementation-plan.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := appaisession.New(appaisession.NewSettingsRepository(filepath.Join(t.TempDir(), "ai-settings.yaml")))
	handler := New(nil, nil, nil, nil, nil, nil, nil).WithAISessions(service).Routes()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/ai/providers/codex/capabilities", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var catalog appaisession.ProviderCapabilityCatalog
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Provider != "codex" || len(catalog.Skills) == 0 || !catalog.SupportsPromptFallback || catalog.Skills[0].SourcePath == "" {
		t.Fatalf("catalog = %#v", catalog)
	}
}

func TestAIRoutesAreUnavailableWithoutService(t *testing.T) {
	handler := New(nil, nil, nil, nil, nil, nil, nil).Routes()
	for _, endpoint := range []string{"/api/ai/settings", "/api/ai/capabilities"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, endpoint, nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status = %d", endpoint, response.Code)
		}
	}
}

func TestGinTransportFallsBackToLegacyRoutes(t *testing.T) {
	handler := New(nil, nil, nil, nil, nil, nil, nil).Routes()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/ai/settings", nil)
	request.Header.Set(requestIDHeader, "request-1")

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get(requestIDHeader) != "request-1" {
		t.Fatalf("request id header = %q", response.Header().Get(requestIDHeader))
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("content type = %q", contentType)
	}
}

func TestGinHealthRoutePreservesContract(t *testing.T) {
	handler := New(nil, nil, nil, nil, nil, nil, nil).Routes()
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("content type = %q", contentType)
	}
	var payload map[string]bool
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload["ok"] {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestAILaunchRouteValidatesBodyAndReportsUnavailableLauncher(t *testing.T) {
	service := appaisession.New(appaisession.NewSettingsRepository(filepath.Join(t.TempDir(), "ai-settings.yaml")))
	handler := New(nil, nil, nil, nil, nil, nil, nil).WithAISessions(service).Routes()

	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodPost, "/api/items/item-1/ai-sessions", strings.NewReader(`{"contextMode":`)))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, body = %s", invalid.Code, invalid.Body.String())
	}

	unavailable := httptest.NewRecorder()
	handler.ServeHTTP(unavailable, httptest.NewRequest(http.MethodPost, "/api/items/item-1/ai-sessions", strings.NewReader(`{"provider":"codex","terminal":"terminal","contextMode":"card_context"}`)))
	if unavailable.Code != http.StatusInternalServerError || !strings.Contains(unavailable.Body.String(), `"code":"launch_failed"`) {
		t.Fatalf("unavailable status = %d, body = %s", unavailable.Code, unavailable.Body.String())
	}

	acceptsSurface := httptest.NewRecorder()
	handler.ServeHTTP(acceptsSurface, httptest.NewRequest(http.MethodPost, "/api/items/item-1/ai-sessions", strings.NewReader(`{"provider":"codex","terminal":"terminal","contextMode":"card_context","surface":"external"}`)))
	if acceptsSurface.Code != http.StatusInternalServerError || !strings.Contains(acceptsSurface.Body.String(), `"code":"launch_failed"`) {
		t.Fatalf("acceptsSurface status = %d, body = %s", acceptsSurface.Code, acceptsSurface.Body.String())
	}
}

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
			MetadataSource: "plan.yaml",
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
	if item.ID != "item-1" || item.Identifier != "PM-003" || item.Status != models.StatusDraft || item.MetadataSource != "plan.yaml" {
		t.Fatalf("unexpected item response: %+v", item)
	}
	if item.Tags == nil {
		t.Fatal("tags should be normalized to an empty array")
	}
}

func TestItemVerificationTestsRoutesPersistSelection(t *testing.T) {
	apiHandler, workspace, idx, _ := reliabilityTestAPI(t)
	writeAPITestFile(t, workspace.Path, "plans/platform/PM-029/plan.yaml", "plan:\n  status: draft\n")
	writeAPITestFile(t, workspace.Path, "plans/platform/PM-029/README.md", "# PM-029\n")
	itemID := stableAPITestPlanID(workspace.ID, "main", "plans/platform/PM-029")
	updatedAt := time.Date(2026, 7, 11, 1, 2, 3, 0, time.UTC)
	if err := idx.ReplaceWorkspace(workspace.ID, []models.ItemDetail{{
		ItemSummary: models.ItemSummary{
			ID:             itemID,
			WorkspaceID:    workspace.ID,
			WorkspaceName:  workspace.Name,
			Branch:         "main",
			Scope:          "platform",
			Identifier:     "PM-029",
			Title:          "Automation runner",
			Status:         models.StatusDraft,
			UpdatedAt:      updatedAt,
			MetadataSource: "plan.yaml",
			ItemPath:       "plans/platform/PM-029",
		},
	}}, nil, updatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := apiHandler.items.Detail(itemID); err != nil {
		t.Fatalf("fixture item detail: %v", err)
	}

	put := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/api/items/"+itemID+"/verification-tests", strings.NewReader(`{"selectedSpecs":["cypress/e2e/create.cy.ts"],"environment":"local"}`)))
	if put.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", put.Code, put.Body.String())
	}

	get := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/items/"+itemID+"/verification-tests", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", get.Code, get.Body.String())
	}
	var payload models.ItemVerificationTests
	if err := json.Unmarshal(get.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Selection.SelectedSpecs) != 1 || payload.Selection.SelectedSpecs[0] != "cypress/e2e/create.cy.ts" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestStateRoutePreservesCountsAndVersionContract(t *testing.T) {
	apiHandler, workspace, idx, _ := reliabilityTestAPI(t)

	first := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/state", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", first.Code, first.Body.String())
	}
	var emptyState map[string]any
	if err := json.Unmarshal(first.Body.Bytes(), &emptyState); err != nil {
		t.Fatal(err)
	}
	if emptyState["version"] == "" || emptyState["workspaceCount"].(float64) != 1 || emptyState["itemCount"].(float64) != 0 || emptyState["updatedAt"] == "" {
		t.Fatalf("unexpected empty state: %#v", emptyState)
	}

	updatedAt := time.Date(2026, 6, 21, 2, 3, 4, 0, time.UTC)
	item := models.ItemDetail{ItemSummary: models.ItemSummary{
		ID:             "item-state",
		WorkspaceID:    workspace.ID,
		WorkspaceName:  workspace.Name,
		Branch:         "main",
		Scope:          "platform",
		Identifier:     "PM-015",
		Title:          "State Contract",
		Status:         models.StatusDraft,
		UpdatedAt:      updatedAt,
		MetadataSource: "plan.yaml",
	}}
	if err := idx.ReplaceWorkspace(workspace.ID, []models.ItemDetail{item}, nil, updatedAt); err != nil {
		t.Fatal(err)
	}

	second := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/api/state", nil))
	var indexedState map[string]any
	if err := json.Unmarshal(second.Body.Bytes(), &indexedState); err != nil {
		t.Fatal(err)
	}
	if second.Code != http.StatusOK || indexedState["workspaceCount"].(float64) != 1 || indexedState["itemCount"].(float64) != 1 {
		t.Fatalf("status = %d, state = %#v", second.Code, indexedState)
	}
	if indexedState["version"] == emptyState["version"] {
		t.Fatal("state version should change when indexed items change")
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

func TestJiraConnectionRouteRequiresServiceAndValidBody(t *testing.T) {
	unavailable := httptest.NewRecorder()
	New(nil, nil, nil, nil, nil, nil, nil).Routes().ServeHTTP(unavailable, httptest.NewRequest(http.MethodPost, "/api/workspaces/w1/jira/test", strings.NewReader(`{}`)))
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable status = %d", unavailable.Code)
	}
}

func TestWorkspaceJiraIssueRouteRequiresService(t *testing.T) {
	unavailable := httptest.NewRecorder()
	New(nil, nil, nil, nil, nil, nil, nil).Routes().ServeHTTP(unavailable, httptest.NewRequest(http.MethodGet, "/api/workspaces/w1/jira/issues/DI-1", nil))
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable status = %d", unavailable.Code)
	}
}

func TestAIPresetsRouteRequiresService(t *testing.T) {
	unavailable := httptest.NewRecorder()
	New(nil, nil, nil, nil, nil, nil, nil).Routes().ServeHTTP(unavailable, httptest.NewRequest(http.MethodGet, "/api/ai/presets", nil))
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable status = %d", unavailable.Code)
	}
}

func TestJiraAttachmentResponseHelpersAreSafe(t *testing.T) {
	if !safeInlineMediaType("image/png") || safeInlineMediaType("text/html") || safeInlineMediaType("application/pdf") {
		t.Fatal("unexpected inline media policy")
	}
	if got := sanitizeDownloadName(`../bad\"name.html`); strings.ContainsAny(got, "/\\\"") || got == "" {
		t.Fatalf("sanitized name=%q", got)
	}
}

func TestCreateWorkspaceSupportsRemoteClonePayload(t *testing.T) {
	remote := t.TempDir()
	if output, err := exec.Command("git", "init", "-b", "main", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init remote: %v: %s", err, output)
	}
	if err := os.MkdirAll(filepath.Join(remote, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remote, "plans", "README.md"), []byte("# Remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", remote, "add", ".").CombinedOutput(); err != nil {
		t.Fatalf("git add remote: %v: %s", err, output)
	}
	commit := exec.Command("git", "-C", remote, "commit", "-m", "seed")
	commit.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit remote: %v: %s", err, output)
	}

	git := gitadapter.New()
	reg := registry.New(filepath.Join(t.TempDir(), "workspaces.yaml"), git)
	idx := itemindex.New(filepath.Join(t.TempDir(), "item-index.yaml"))
	handler := New(reg, idx, nil, nil, nil, git, nil)
	cloneRoot := t.TempDir()
	body := `{"name":"Remote","registrationMode":"remote_clone","remoteUrl":"file://` + remote + `","cloneRoot":"` + cloneRoot + `","baselineBranch":"main","sources":["plans"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces", strings.NewReader(body))
	res := httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var payload struct {
		Workspace    models.WorkspaceConfig `json:"workspace"`
		OperationLog string                 `json:"operationLog"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	workspace := payload.Workspace
	if workspace.RegistrationMode != models.WorkspaceRegistrationModeRemoteClone || workspace.RemoteURL != "file://"+remote || !workspace.ClonePathManaged {
		t.Fatalf("workspace = %+v", workspace)
	}
	if strings.TrimSpace(payload.OperationLog) == "" {
		t.Fatalf("expected clone operation log in response: %s", res.Body.String())
	}
	resolvedCloneRoot, _ := filepath.EvalSymlinks(cloneRoot)
	resolvedWorkspacePath, _ := filepath.EvalSymlinks(workspace.Path)
	if !strings.HasPrefix(resolvedWorkspacePath, resolvedCloneRoot) {
		t.Fatalf("workspace path = %q (%q) cloneRoot = %q (%q)", workspace.Path, resolvedWorkspacePath, cloneRoot, resolvedCloneRoot)
	}
}

func TestWorkspaceImportPreviewEndpointReturnsCandidatesWithoutWriting(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	commit := exec.Command("git", "-C", root, "commit", "--allow-empty", "-m", "init")
	commit.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	source := filepath.Join(t.TempDir(), "workspaces.yaml")
	if err := os.WriteFile(source, []byte("- name: Imported\n  path: "+root+"\n  baselineBranch: main\n  sources: [plans]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	registryPath := filepath.Join(dataDir, "workspaces.yaml")
	git := gitadapter.New()
	reg := registry.New(registryPath, git)
	handler := New(reg, itemindex.New(filepath.Join(dataDir, "items.yaml")), nil, nil, nil, git, nil).Routes()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/workspaces/import-preview", strings.NewReader(`{"sourcePath":"`+source+`"}`)))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var preview models.WorkspaceImportPreview
	if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Candidates) != 1 || preview.Candidates[0].Status != "valid" || preview.DestinationPath != registryPath {
		t.Fatalf("preview = %+v", preview)
	}
	if _, err := os.Stat(registryPath); !os.IsNotExist(err) {
		t.Fatalf("preview wrote registry: %v", err)
	}
	importResponse := httptest.NewRecorder()
	importBody, err := json.Marshal(models.WorkspaceImportRequest{SourcePath: source, CandidateKeys: []string{preview.Candidates[0].CandidateKey}})
	if err != nil {
		t.Fatal(err)
	}
	handler.ServeHTTP(importResponse, httptest.NewRequest(http.MethodPost, "/api/workspaces/import", strings.NewReader(string(importBody))))
	if importResponse.Code != http.StatusOK || !strings.Contains(importResponse.Body.String(), `"status":"scan_failed"`) {
		t.Fatalf("import status = %d body = %s", importResponse.Code, importResponse.Body.String())
	}
	if _, err := os.Stat(registryPath); err != nil {
		t.Fatalf("import did not write registry: %v", err)
	}

	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodPost, "/api/workspaces/import-preview", strings.NewReader(`{"sourcePath":"relative.yaml"}`)))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d body = %s", invalid.Code, invalid.Body.String())
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
	if contentType := auditResponse.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("audit content type = %q", contentType)
	}
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

func TestFileContentRouteReturnsViewerMetadataAndRejectsBinary(t *testing.T) {
	apiHandler, workspace, idx, _ := reliabilityTestAPI(t)
	itemPath := "plans/platform/PM-006"
	itemRoot := filepath.Join(workspace.Path, itemPath)
	if err := os.MkdirAll(itemRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemRoot, "README.md"), []byte("# Viewer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemRoot, "image.bin"), []byte{'P', 0, 'N', 'G'}, 0o644); err != nil {
		t.Fatal(err)
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "item-viewer", WorkspaceID: workspace.ID, ItemPath: itemPath, Title: "PM-006", Identifier: "PM-006", Scope: "platform"}}
	if err := idx.ReplaceWorkspace(workspace.ID, []models.ItemDetail{item}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}

	markdownResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(markdownResponse, httptest.NewRequest(http.MethodGet, "/api/items/item-viewer/files/README_md", nil))
	var content models.FileContent
	if err := json.Unmarshal(markdownResponse.Body.Bytes(), &content); err != nil {
		t.Fatal(err)
	}
	if markdownResponse.Code != http.StatusOK || content.Kind != models.FileKindMarkdown || !content.Editable || content.SizeBytes == 0 {
		t.Fatalf("status = %d, content = %+v", markdownResponse.Code, content)
	}

	binaryResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(binaryResponse, httptest.NewRequest(http.MethodGet, "/api/items/item-viewer/files/image_bin", nil))
	var payload map[string]string
	if err := json.Unmarshal(binaryResponse.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if binaryResponse.Code != http.StatusBadRequest || payload["error"] != fileaccess.ErrUnsupportedContent.Error() {
		t.Fatalf("status = %d, payload = %#v", binaryResponse.Code, payload)
	}
}

func TestWorkspaceTreeAndFileRoutes(t *testing.T) {
	apiHandler, workspace, _, _ := reliabilityTestAPI(t)
	if err := os.WriteFile(filepath.Join(workspace.Path, "README.md"), []byte("# Explorer\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	treeResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(treeResponse, httptest.NewRequest(http.MethodGet, "/api/workspaces/"+workspace.ID+"/tree", nil))
	var listing models.WorkspaceDirectoryListing
	if err := json.Unmarshal(treeResponse.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if treeResponse.Code != http.StatusOK || listing.WorkspaceID != workspace.ID || len(listing.Entries) == 0 {
		t.Fatalf("tree status=%d listing=%#v", treeResponse.Code, listing)
	}

	fileResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(fileResponse, httptest.NewRequest(http.MethodGet, "/api/workspaces/"+workspace.ID+"/files?path=README.md", nil))
	var content models.FileContent
	if err := json.Unmarshal(fileResponse.Body.Bytes(), &content); err != nil {
		t.Fatal(err)
	}
	if fileResponse.Code != http.StatusOK || content.Kind != models.FileKindMarkdown || content.Content != "# Explorer\n" {
		t.Fatalf("file status=%d content=%#v", fileResponse.Code, content)
	}
}

func TestWorkspaceFileSaveDiffRevertAndErrorMapping(t *testing.T) {
	apiHandler, workspace, _, _ := reliabilityTestAPI(t)
	path := filepath.Join(workspace.Path, "README.md")
	if err := os.WriteFile(path, []byte("# Original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit := exec.Command("git", "-C", workspace.Path, "add", "README.md")
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	commit = exec.Command("git", "-C", workspace.Path, "commit", "-m", "add readme")
	commit.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}

	readResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(readResponse, httptest.NewRequest(http.MethodGet, "/api/workspaces/"+workspace.ID+"/files?path=README.md", nil))
	var current models.FileContent
	if err := json.Unmarshal(readResponse.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	saveBody := strings.NewReader(`{"path":"README.md","content":"# Changed\n","expectedHash":"` + current.Hash + `"}`)
	saveResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(saveResponse, httptest.NewRequest(http.MethodPut, "/api/workspaces/"+workspace.ID+"/files", saveBody))
	if saveResponse.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", saveResponse.Code, saveResponse.Body.String())
	}

	diffResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(diffResponse, httptest.NewRequest(http.MethodGet, "/api/workspaces/"+workspace.ID+"/files/diff?path=README.md", nil))
	if diffResponse.Code != http.StatusOK || !strings.Contains(diffResponse.Body.String(), "Changed") {
		t.Fatalf("diff status=%d body=%s", diffResponse.Code, diffResponse.Body.String())
	}

	revertResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(revertResponse, httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/files/revert", strings.NewReader(`{"path":"README.md"}`)))
	if revertResponse.Code != http.StatusOK {
		t.Fatalf("revert status=%d body=%s", revertResponse.Code, revertResponse.Body.String())
	}

	conflictResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(conflictResponse, httptest.NewRequest(http.MethodPut, "/api/workspaces/"+workspace.ID+"/files", strings.NewReader(`{"path":"README.md","content":"x","expectedHash":"stale"}`)))
	if conflictResponse.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d body=%s", conflictResponse.Code, conflictResponse.Body.String())
	}

	missingResponse := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(missingResponse, httptest.NewRequest(http.MethodGet, "/api/workspaces/missing/files?path=README.md", nil))
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("missing status=%d body=%s", missingResponse.Code, missingResponse.Body.String())
	}
}

func TestWorkspaceProductivityRoutes(t *testing.T) {
	apiHandler, workspace, _, auditStore := reliabilityTestAPI(t)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		apiHandler.Routes().ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
		return response
	}

	directory := request(http.MethodPost, "/api/workspaces/"+workspace.ID+"/directories", `{"parentPath":"plans","name":"guides"}`)
	if directory.Code != http.StatusOK {
		t.Fatalf("directory status=%d body=%s", directory.Code, directory.Body.String())
	}
	created := request(http.MethodPost, "/api/workspaces/"+workspace.ID+"/files", `{"parentPath":"plans/guides","name":"start.md","content":"# Start\n"}`)
	if created.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	search := request(http.MethodGet, "/api/workspaces/files/search?q=start&workspaceId="+workspace.ID, "")
	var searchResult models.WorkspacePathSearchResponse
	if err := json.Unmarshal(search.Body.Bytes(), &searchResult); err != nil || search.Code != http.StatusOK || len(searchResult.Results) != 1 {
		t.Fatalf("search status=%d result=%#v err=%v", search.Code, searchResult, err)
	}
	renamed := request(http.MethodPost, "/api/workspaces/"+workspace.ID+"/paths/rename", `{"path":"plans/guides/start.md","destinationPath":"plans/guides/intro.md"}`)
	if renamed.Code != http.StatusOK {
		t.Fatalf("rename status=%d body=%s", renamed.Code, renamed.Body.String())
	}
	conflict := request(http.MethodPost, "/api/workspaces/"+workspace.ID+"/paths/rename", `{"path":"plans/guides/intro.md","destinationPath":"plans/guides/intro.md"}`)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d body=%s", conflict.Code, conflict.Body.String())
	}
	states := request(http.MethodGet, "/api/workspaces/"+workspace.ID+"/git/path-status", "")
	var pathStates []models.WorkspacePathGitState
	if err := json.Unmarshal(states.Body.Bytes(), &pathStates); err != nil || states.Code != http.StatusOK || len(pathStates) == 0 {
		t.Fatalf("states status=%d states=%#v err=%v", states.Code, pathStates, err)
	}
	events, err := auditStore.Recent(10)
	if err != nil || len(events) < 3 {
		t.Fatalf("audit events=%#v err=%v", events, err)
	}
}

func TestContentSearchRoutesAndValidation(t *testing.T) {
	apiHandler, workspace, idx, _ := reliabilityTestAPI(t)
	itemPath := "plans/platform/PM-009"
	if err := os.MkdirAll(filepath.Join(workspace.Path, filepath.FromSlash(itemPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace.Path, filepath.FromSlash(itemPath), "README.md"), []byte("Scoped needle here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace.Path, "root.txt"), []byte("Root needle here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "item-search", WorkspaceID: workspace.ID, WorkspaceName: workspace.Name, ItemPath: itemPath}}
	if err := idx.ReplaceWorkspace(workspace.ID, []models.ItemDetail{item}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}

	request := func(path string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		apiHandler.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		return response
	}
	itemResponse := request("/api/items/item-search/content-search?q=needle")
	var itemResult models.WorkspaceContentSearchResponse
	if err := json.Unmarshal(itemResponse.Body.Bytes(), &itemResult); err != nil || itemResponse.Code != http.StatusOK || len(itemResult.Results) != 1 || itemResult.Results[0].ItemID != "item-search" {
		t.Fatalf("item status=%d result=%#v err=%v", itemResponse.Code, itemResult, err)
	}
	sourcesResponse := request("/api/workspaces/files/content-search?q=needle&mode=sources&workspaceId=" + workspace.ID)
	var sourcesResult models.WorkspaceContentSearchResponse
	if err := json.Unmarshal(sourcesResponse.Body.Bytes(), &sourcesResult); err != nil || sourcesResponse.Code != http.StatusOK || len(sourcesResult.Results) != 1 {
		t.Fatalf("sources status=%d result=%#v err=%v", sourcesResponse.Code, sourcesResult, err)
	}
	allResponse := request("/api/workspaces/files/content-search?q=needle&mode=all&workspaceId=" + workspace.ID + "&caseSensitive=false")
	var allResult models.WorkspaceContentSearchResponse
	if err := json.Unmarshal(allResponse.Body.Bytes(), &allResult); err != nil || allResponse.Code != http.StatusOK || len(allResult.Results) != 2 {
		t.Fatalf("all status=%d result=%#v err=%v", allResponse.Code, allResult, err)
	}
	for _, path := range []string{
		"/api/items/missing/content-search?q=needle",
		"/api/workspaces/files/content-search?q=needle&workspaceId=missing",
	} {
		if response := request(path); response.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{
		"/api/workspaces/files/content-search?q=x",
		"/api/workspaces/files/content-search?q=needle&mode=invalid",
		"/api/workspaces/files/content-search?q=needle&caseSensitive=perhaps",
	} {
		if response := request(path); response.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}

	pathSearch := request("/api/workspaces/files/search?q=README&workspaceId=" + workspace.ID)
	if pathSearch.Code != http.StatusOK {
		t.Fatalf("path search regression status=%d body=%s", pathSearch.Code, pathSearch.Body.String())
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

func TestGitBranchesReturnsCurrentSortedLocalBranches(t *testing.T) {
	apiHandler, workspace, _, _ := reliabilityTestAPI(t)
	for _, branch := range []string{"zeta", "alpha"} {
		if output, err := exec.Command("git", "-C", workspace.Path, "branch", branch).CombinedOutput(); err != nil {
			t.Fatalf("create branch %q: %v: %s", branch, err, output)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/api/workspaces/"+workspace.ID+"/git/branches", nil)
	response := httptest.NewRecorder()
	apiHandler.Routes().ServeHTTP(response, request)

	var payload models.WorkspaceBranches
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	want := []string{"alpha", "main", "zeta"}
	if response.Code != http.StatusOK || payload.WorkspaceID != workspace.ID || payload.Current != "main" || len(payload.Branches) != len(want) {
		t.Fatalf("status = %d, payload = %#v", response.Code, payload)
	}
	for index := range want {
		if payload.Branches[index] != want[index] {
			t.Fatalf("branch %d = %q, want %q", index, payload.Branches[index], want[index])
		}
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
	apiHandler.Routes().ServeHTTP(createdResponse, httptest.NewRequest(http.MethodPost, "/api/saved-filters", strings.NewReader(`{"name":"Drafts","route":"/workstream","filters":{"statuses":["draft"]}}`)))
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
	healthService := workspacehealth.NewHealthService(reg, idx, git)
	files := fileaccess.New()
	scan := scanner.New(git)
	itemWriter := itemwriter.New(files, scan, idx, reg)
	return NewWithReliability(reg, idx, scan, files, itemWriter, git, nil, auditStore, healthService), workspace, idx, auditStore
}

func writeAPITestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stableAPITestPlanID(repoID, branch, relItemPath string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(repoID + "|" + branch + "|" + relItemPath))
	return fmt.Sprintf("%s-%08x", repoID, h.Sum32())
}
