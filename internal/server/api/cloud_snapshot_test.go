package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kode-stream/internal/common/models"
	"kode-stream/internal/provider"
)

func TestCloudSnapshotReadsCommitPinnedProviderContent(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/repos":
			_, _ = w.Write([]byte(`[{"id":1,"name":"repo","full_name":"acme/repo"}]`))
		case "/repos/acme/repo/branches":
			_, _ = w.Write([]byte(`[{"name":"main","commit":{"sha":"commit-1"}}]`))
		case "/repos/acme/repo/tags":
			_, _ = w.Write([]byte(`[]`))
		case "/repos/acme/repo/git/trees/commit-1":
			_, _ = w.Write([]byte(`{"tree":[{"path":"README.md","type":"blob","sha":"file-1","size":8}]}`))
		case "/repos/acme/repo/contents/README.md":
			_, _ = w.Write([]byte(`{"path":"README.md","content":"c25hcHNob3Q=","encoding":"base64"}`))
		default:
			t.Fatalf("unexpected provider request %s", r.URL.Path)
		}
	}))
	defer providerServer.Close()

	apiHandler, _, _, _ := reliabilityTestAPI(t)
	apiHandler = apiHandler.WithRuntimeConfig(testCloudRuntimeConfig())
	if err := apiHandler.cloudProviders.registry.Upsert(provider.Instance{ID: "github", Name: "GitHub", Kind: "github", BaseURL: providerServer.URL}); err != nil {
		t.Fatal(err)
	}
	userID := stableCloudUserID("editor")
	if err := apiHandler.cloudProviders.connections.Save(userID, "github", "read-token"); err != nil {
		t.Fatal(err)
	}
	workspace := apiHandler.cloudWorkspaces.Upsert(models.WorkspaceConfig{ID: "snapshot", Name: "Snapshot", OwnerUserID: userID, AccessMode: models.WorkspaceAccessModeRemoteSnapshot, Provider: "github", ProviderInstanceID: "github", ProviderRepository: "acme/repo", SelectedRef: "main", Sources: []string{}})
	handler := apiHandler.Routes()

	for _, endpoint := range []string{"/api/workspaces/" + workspace.ID + "/snapshot", "/api/workspaces/" + workspace.ID + "/snapshot/tree", "/api/workspaces/" + workspace.ID + "/snapshot/files?path=README.md"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, endpoint, nil)
		request.Header.Set("X-Kode-Stream-Subject", "editor")
		request.Header.Set("X-Kode-Stream-Role", "editor")
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "commit-1") {
			t.Fatalf("%s: status = %d body = %s", endpoint, response.Code, response.Body.String())
		}
	}
}

func TestCleanSnapshotPathRejectsTraversal(t *testing.T) {
	for _, value := range []string{"../secret", "plans/../secret", "/../../secret"} {
		if got := cleanSnapshotPath(value); got != "" {
			t.Fatalf("cleanSnapshotPath(%q) = %q", value, got)
		}
	}
}

func TestCloudRegistersRemoteSnapshotWithoutAgent(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/repos":
			_, _ = w.Write([]byte(`[{"id":1,"full_name":"acme/repo"}]`))
		case "/repos/acme/repo/branches":
			_, _ = w.Write([]byte(`[{"name":"main","commit":{"sha":"commit-1"}}]`))
		case "/repos/acme/repo/tags":
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer providerServer.Close()
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	apiHandler = apiHandler.WithRuntimeConfig(testCloudRuntimeConfig())
	if err := apiHandler.cloudProviders.registry.Upsert(provider.Instance{ID: "github", Name: "GitHub", Kind: "github", BaseURL: providerServer.URL}); err != nil {
		t.Fatal(err)
	}
	if err := apiHandler.cloudProviders.connections.Save(stableCloudUserID("editor"), "github", "read-token"); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workspaces", strings.NewReader(`{"name":"Snapshot","accessMode":"remote_snapshot","provider":"github","providerInstanceId":"github","providerRepository":"acme/repo","selectedRef":"main","sources":["plans"]}`))
	request.Header.Set("X-Kode-Stream-Subject", "editor")
	request.Header.Set("X-Kode-Stream-Role", "editor")
	request.Header.Set(csrfHeader, stableCloudUserID("editor:csrf"))
	apiHandler.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), "commit-1") || strings.Contains(response.Body.String(), "agentId") {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}
