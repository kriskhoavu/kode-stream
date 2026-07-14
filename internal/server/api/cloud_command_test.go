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

func TestCloudCommandRoutesToOwnerAgentAndRedactsLog(t *testing.T) {
	apiHandler, workspaceID := cloudCommandTestAPI(t, true)
	handler := apiHandler.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspaceID+"/commands", strings.NewReader(`{"type":"git","payload":{"op":"status"},"log":"token=abc123 password=hunter2 ok"}`))
	request.Header.Set("X-Kode-Stream-Subject", "editor")
	request.Header.Set("X-Kode-Stream-Role", "editor")
	request.Header.Set(csrfHeader, stableCloudUserID("editor:csrf"))
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var result models.CommandResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Accepted || result.Command.AgentID != "agent-1" || result.Command.UserID != stableCloudUserID("editor") || result.Command.Capability != models.CapabilityGit {
		t.Fatalf("result = %#v", result)
	}
	if strings.Contains(result.Log, "abc123") || strings.Contains(result.Log, "hunter2") || !strings.Contains(result.Log, "[REDACTED]") {
		t.Fatalf("log not redacted: %q", result.Log)
	}
}

func TestCloudCommandRejectsOfflineAgent(t *testing.T) {
	apiHandler, workspaceID := cloudCommandTestAPI(t, false)
	handler := apiHandler.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspaceID+"/commands", strings.NewReader(`{"type":"git"}`))
	request.Header.Set("X-Kode-Stream-Subject", "editor")
	request.Header.Set("X-Kode-Stream-Role", "editor")
	request.Header.Set(csrfHeader, stableCloudUserID("editor:csrf"))
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestCloudCommandRejectsViewerWriteCapability(t *testing.T) {
	apiHandler, workspaceID := cloudCommandTestAPI(t, true)
	handler := apiHandler.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspaceID+"/commands", strings.NewReader(`{"type":"file"}`))
	request.Header.Set("X-Kode-Stream-Subject", "editor")
	request.Header.Set("X-Kode-Stream-Role", "viewer")
	request.Header.Set(csrfHeader, stableCloudUserID("editor:csrf"))
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestCloudDeniesHostedExecutionRoutes(t *testing.T) {
	apiHandler, workspaceID := cloudCommandTestAPI(t, true)
	handler := apiHandler.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspaceID+"/git/pull", strings.NewReader(`{}`))
	request.Header.Set("X-Kode-Stream-Subject", "editor")
	request.Header.Set("X-Kode-Stream-Role", "editor")
	request.Header.Set(csrfHeader, stableCloudUserID("editor:csrf"))
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func cloudCommandTestAPI(t *testing.T, connected bool) (*API, string) {
	t.Helper()
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	apiHandler = apiHandler.WithRuntimeConfig(testCloudRuntimeConfig())
	userID := stableCloudUserID("editor")
	workspace := models.WorkspaceConfig{ID: "ws-command", Name: "Command", OwnerUserID: userID, AgentID: "agent-1", Location: models.WorkspaceLocationCloudAgent, Sources: []string{}}
	apiHandler.cloudWorkspaces.Upsert(workspace)
	status := "offline"
	if connected {
		status = "connected"
	}
	apiHandler.agentStore.Upsert(models.CloudAgent{ID: "agent-1", UserID: userID, Name: "MacBook", Status: status, LastSeenAt: time.Now().UTC()})
	return apiHandler, workspace.ID
}
