package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"kode-stream/internal/common/models"
)

func TestCloudAgentConnectTokenRequiresAuthenticatedUserAndCSRF(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := apiHandler.WithRuntimeConfig(testCloudRuntimeConfig()).Routes()

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/api/agents/connect-token", strings.NewReader(`{}`)))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	forbidden := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/agents/connect-token", strings.NewReader(`{}`))
	request.Header.Set("X-Kode-Stream-Subject", "editor")
	request.Header.Set("X-Kode-Stream-Role", "editor")
	handler.ServeHTTP(forbidden, request)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("forbidden status = %d body=%s", forbidden.Code, forbidden.Body.String())
	}
}

func TestCloudAgentTokenExpiry(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	apiHandler = apiHandler.WithRuntimeConfig(testCloudRuntimeConfig())
	expired := apiHandler.signAgentToken(agentConnectToken{UserID: "user", AgentID: "agent", ExpiresAt: time.Now().UTC().Add(-time.Second)})
	if _, ok := apiHandler.verifyAgentToken(expired); ok {
		t.Fatal("expired token verified")
	}
}

func TestCloudAgentStoreScopesAgentsByUser(t *testing.T) {
	now := time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC)
	store := newCloudAgentStore(func() time.Time { return now })
	store.Upsert(models.CloudAgent{ID: "agent-a", UserID: "user-a", Name: "A", Status: "connected"})
	store.Upsert(models.CloudAgent{ID: "agent-b", UserID: "user-b", Name: "B", Status: "connected"})

	agents := store.List("user-a")
	if len(agents) != 1 || agents[0].ID != "agent-a" {
		t.Fatalf("agents = %#v", agents)
	}
}

func TestCloudAgentChannelAuthenticatesAndTracksConnection(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	apiHandler = apiHandler.WithRuntimeConfig(testCloudRuntimeConfig())
	server := httptest.NewServer(apiHandler.Routes())
	defer server.Close()

	badURL := websocketURL(server.URL, "/api/agents/channel?token=bad")
	if conn, _, err := websocket.DefaultDialer.Dial(badURL, nil); err == nil {
		_ = conn.Close()
		t.Fatal("bad token connected")
	}

	token := apiHandler.signAgentToken(agentConnectToken{UserID: "user-1", AgentID: "agent-1", Name: "MacBook", Platform: "darwin", ExpiresAt: time.Now().UTC().Add(time.Minute)})
	conn, response, err := websocket.DefaultDialer.Dial(websocketURL(server.URL, "/api/agents/channel?token="+url.QueryEscape(token)), nil)
	if err != nil {
		t.Fatalf("dial status=%v err=%v", response, err)
	}
	defer conn.Close()

	var connected struct {
		Type  string            `json:"type"`
		Agent models.CloudAgent `json:"agent"`
	}
	if err := conn.ReadJSON(&connected); err != nil {
		t.Fatal(err)
	}
	if connected.Type != "connected" || connected.Agent.Status != "connected" || connected.Agent.UserID != "user-1" {
		t.Fatalf("connected = %#v", connected)
	}
	if err := conn.WriteJSON(map[string]string{"type": "heartbeat"}); err != nil {
		t.Fatal(err)
	}
	var heartbeat map[string]any
	if err := conn.ReadJSON(&heartbeat); err != nil {
		t.Fatal(err)
	}
	if heartbeat["type"] != "heartbeat_ack" {
		data, _ := json.Marshal(heartbeat)
		t.Fatalf("heartbeat = %s", data)
	}
	agents := apiHandler.agentStore.List("user-1")
	if len(agents) != 1 || agents[0].Status != "connected" {
		t.Fatalf("agents = %#v", agents)
	}
}

func websocketURL(serverURL, path string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http") + path
}
