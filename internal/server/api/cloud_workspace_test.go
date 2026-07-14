package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/common/models"
)

func TestCloudWorkspaceRegistrationFromAgentStoresMetadata(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	apiHandler = apiHandler.WithRuntimeConfig(testCloudRuntimeConfig())
	handler := apiHandler.Routes()
	token := apiHandler.signAgentToken(agentConnectToken{UserID: "user-1", AgentID: "agent-1", Name: "MacBook", ExpiresAt: time.Now().UTC().Add(time.Minute)})

	create := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/from-agent", strings.NewReader(`{"name":"Platform","baselineBranch":"main","sources":["plans"],"remoteUrl":"git@example.com:repo.git","localRootLabel":"/Users/kdvu/src/repo","publishedSummary":true}`))
	request.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(create, request)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", create.Code, create.Body.String())
	}
	var workspace models.WorkspaceConfig
	if err := json.Unmarshal(create.Body.Bytes(), &workspace); err != nil {
		t.Fatal(err)
	}
	if workspace.Location != models.WorkspaceLocationCloudAgent || workspace.OwnerUserID != "user-1" || workspace.AgentID != "agent-1" || workspace.Path != "" || workspace.LocalRootLabel != ".../repo" {
		t.Fatalf("workspace = %#v", workspace)
	}
}

func TestCloudWorkspacesAreScopedToSessionUser(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	apiHandler = apiHandler.WithRuntimeConfig(testCloudRuntimeConfig())
	apiHandler.cloudWorkspaces.Upsert(models.WorkspaceConfig{ID: "ws-a", Name: "A", OwnerUserID: stableCloudUserID("user-a"), Location: models.WorkspaceLocationCloudAgent, AgentID: "agent-a", Sources: []string{}})
	apiHandler.cloudWorkspaces.Upsert(models.WorkspaceConfig{ID: "ws-b", Name: "B", OwnerUserID: stableCloudUserID("user-b"), Location: models.WorkspaceLocationCloudAgent, AgentID: "agent-b", Sources: []string{}})
	handler := apiHandler.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workspaces", nil)
	request.Header.Set("X-Kode-Stream-Subject", "user-a")
	request.Header.Set("X-Kode-Stream-Role", "viewer")
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var workspaces []models.WorkspaceConfig
	if err := json.Unmarshal(response.Body.Bytes(), &workspaces); err != nil {
		t.Fatal(err)
	}
	if len(workspaces) != 1 || workspaces[0].ID != "ws-a" {
		t.Fatalf("workspaces = %#v", workspaces)
	}
}

func TestCloudRejectsBrowserLocalPathAndRemoteClone(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := apiHandler.WithRuntimeConfig(testCloudRuntimeConfig()).Routes()

	for _, body := range []string{
		`{"name":"Direct","path":"/tmp/repo","baselineBranch":"main","sources":["plans"]}`,
		`{"name":"Clone","remoteUrl":"https://example.com/repo.git","registrationMode":"remote_clone","baselineBranch":"main","sources":["plans"]}`,
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/workspaces", strings.NewReader(body))
		request.Header.Set("X-Kode-Stream-Subject", "editor")
		request.Header.Set("X-Kode-Stream-Role", "editor")
		request.Header.Set(csrfHeader, stableCloudUserID("editor:csrf"))
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
		}
	}
}

func TestCloudWorkspaceListShowsOfflineAgentMetadata(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	apiHandler = apiHandler.WithRuntimeConfig(testCloudRuntimeConfig())
	userID := stableCloudUserID("viewer")
	apiHandler.cloudWorkspaces.Upsert(models.WorkspaceConfig{ID: "ws-offline", Name: "Offline", OwnerUserID: userID, Location: models.WorkspaceLocationCloudAgent, AgentID: "agent-offline", ScanStatus: "offline", Sources: []string{}})
	handler := apiHandler.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workspaces", nil)
	request.Header.Set("X-Kode-Stream-Subject", "viewer")
	request.Header.Set("X-Kode-Stream-Role", "viewer")
	handler.ServeHTTP(response, request)

	var workspaces []models.WorkspaceConfig
	if err := json.Unmarshal(response.Body.Bytes(), &workspaces); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || len(workspaces) != 1 || workspaces[0].ScanStatus != "offline" {
		t.Fatalf("status=%d workspaces=%#v", response.Code, workspaces)
	}
}
