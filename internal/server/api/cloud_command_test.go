package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/audit"
	"kode-stream/internal/common/models"
)

func TestCloudCommandIsUnavailableUntilDeliveryLifecycleExists(t *testing.T) {
	apiHandler, workspaceID := cloudCommandTestAPI(t, true)
	handler := apiHandler.Routes()

	response := postAsCloudEditor(handler, "/api/workspaces/"+workspaceID+"/commands", `{"type":"git","payload":{"op":"status"},"log":"token=abc123 password=hunter2 ok"}`, "editor")

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	events, err := apiHandler.cloud.audit.QueryContext(context.Background(), audit.Query{OwnerUserID: stableCloudUserID("editor"), WorkspaceID: workspaceID, Limit: 1})
	if err != nil || len(events) != 1 || events[0].ActorUserID != stableCloudUserID("editor") || events[0].Status != models.AuditStatusFailed {
		t.Fatalf("command audit = %#v, %v", events, err)
	}
	raw, _ := json.Marshal(events[0])
	if strings.Contains(string(raw), "abc123") || strings.Contains(string(raw), "hunter2") || strings.Contains(string(raw), `\"op\"`) {
		t.Fatalf("audit disclosed command data: %s", raw)
	}
}

func TestCloudCommandRejectsOfflineAgent(t *testing.T) {
	apiHandler, workspaceID := cloudCommandTestAPI(t, false)
	handler := apiHandler.Routes()

	response := postAsCloudEditor(handler, "/api/workspaces/"+workspaceID+"/commands", `{"type":"git"}`, "editor")

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	events, err := apiHandler.cloud.audit.QueryContext(context.Background(), audit.Query{OwnerUserID: stableCloudUserID("editor"), WorkspaceID: workspaceID, Limit: 1})
	if err != nil || len(events) != 1 || events[0].Status != models.AuditStatusFailed {
		t.Fatalf("failed command audit = %#v, %v", events, err)
	}
}

func TestCloudCommandRejectsViewerWriteCapability(t *testing.T) {
	apiHandler, workspaceID := cloudCommandTestAPI(t, true)
	handler := apiHandler.Routes()

	response := postAsCloudEditor(handler, "/api/workspaces/"+workspaceID+"/commands", `{"type":"file"}`, "viewer")

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestCloudCommandRejectsRemoteSnapshotExecutionWithoutProviderCall(t *testing.T) {
	apiHandler, _ := cloudCommandTestAPI(t, true)
	userID := stableCloudUserID("editor")
	workspace, err := apiHandler.cloud.workspaces.Upsert(context.Background(), models.WorkspaceConfig{
		ID:                 "ws-snapshot",
		Name:               "Snapshot",
		OwnerUserID:        userID,
		AccessMode:         models.WorkspaceAccessModeRemoteSnapshot,
		Provider:           "github",
		ProviderInstanceID: "github",
		ProviderRepository: "acme/repo",
		SelectedRef:        "main",
		Sources: models.SourceList(),
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := apiHandler.Routes()

	response := postAsCloudEditor(handler, "/api/workspaces/"+workspace.ID+"/commands", `{"type":"git"}`, "editor")

	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "read-only snapshot") {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestCloudDeniesHostedExecutionRoutes(t *testing.T) {
	apiHandler, workspaceID := cloudCommandTestAPI(t, true)
	handler := apiHandler.Routes()

	response := postAsCloudEditor(handler, "/api/workspaces/"+workspaceID+"/git/pull", `{}`, "editor")

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func cloudCommandTestAPI(t *testing.T, connected bool) (*API, string) {
	t.Helper()
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	apiHandler = withTestRuntime(apiHandler, testCloudRuntimeConfig())
	userID := stableCloudUserID("editor")
	workspace := models.WorkspaceConfig{ID: "ws-command", Name: "Command", OwnerUserID: userID, AgentID: "agent-1", Location: models.WorkspaceLocationCloudAgent, Sources: models.SourceList()}
	if _, err := apiHandler.cloud.workspaces.Upsert(context.Background(), workspace); err != nil {
		t.Fatal(err)
	}
	status := "offline"
	if connected {
		status = "connected"
	}
	if _, err := apiHandler.cloud.agents.Upsert(context.Background(), models.CloudAgent{ID: "agent-1", UserID: userID, Name: "MacBook", Status: status, LastSeenAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	return apiHandler, workspace.ID
}

// postAsCloudEditor posts body to path as the Cloud user "editor" with role,
// carrying that user's CSRF token, and returns the recorded response.
func postAsCloudEditor(handler http.Handler, path, body, role string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("X-Kode-Stream-Subject", "editor")
	request.Header.Set("X-Kode-Stream-Role", role)
	request.Header.Set(csrfHeader, stableCloudUserID("editor:csrf"))
	handler.ServeHTTP(response, request)
	return response
}
