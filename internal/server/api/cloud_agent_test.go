package api

import (
	"context"
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

func TestCloudAgentConnectTokenRequiresAuthenticatedUser(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := withTestRuntime(apiHandler, testCloudRuntimeConfig()).Routes()

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/api/agents/connect-token", strings.NewReader(`{}`)))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	authorized := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/agents/connect-token", strings.NewReader(`{}`))
	request.Header.Set("X-Auth-Request-User", "editor")
	request.Header.Set("X-Auth-Request-Email", "editor@example.com")
	handler.ServeHTTP(authorized, request)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorized status = %d body=%s", authorized.Code, authorized.Body.String())
	}
}

func TestCloudAgentConnectTokenUsesTrustedProxyIdentity(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := withTestRuntime(apiHandler, testAppOIDCRuntimeConfig()).Routes()

	forbidden := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/agents/connect-token", strings.NewReader(`{}`))
	request.Header.Set("X-Kode-Stream-Subject", "editor")
	request.Header.Set("X-Kode-Stream-Role", "editor")
	handler.ServeHTTP(forbidden, request)
	if forbidden.Code != http.StatusOK {
		t.Fatalf("trusted proxy status = %d body=%s", forbidden.Code, forbidden.Body.String())
	}
}

func TestCloudAgentTokenExpiry(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	apiHandler = withTestRuntime(apiHandler, testCloudRuntimeConfig())
	expired := apiHandler.cloud.signAgentToken(agentConnectToken{UserID: "user", AgentID: "agent", ExpiresAt: time.Now().UTC().Add(-time.Second)})
	if _, ok := apiHandler.cloud.verifyAgentToken(expired); ok {
		t.Fatal("expired token verified")
	}
}

func TestCloudAgentEnrollmentTokenCannotBeReplayed(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	apiHandler = withTestRuntime(apiHandler, testCloudRuntimeConfig())
	server := httptest.NewServer(apiHandler.Routes())
	defer server.Close()
	token := apiHandler.cloud.signAgentToken(agentConnectToken{ID: "single-use", UserID: "user", AgentID: "agent", Name: "Agent", ExpiresAt: time.Now().UTC().Add(time.Minute)})
	endpoint := websocketURL(server.URL, "/api/agents/channel?token="+url.QueryEscape(token))
	first, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = first.Close()
	if replay, response, err := websocket.DefaultDialer.Dial(endpoint, nil); err == nil {
		_ = replay.Close()
		t.Fatal("replayed enrollment token connected")
	} else if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("replay response=%v err=%v", response, err)
	}
}

func TestCloudAgentConnectTokenRejectsUnknownTrailingAndOversizedBodies(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	handler := withTestRuntime(apiHandler, testCloudRuntimeConfig()).Routes()
	for _, body := range []string{`{"unknown":true}`, `{} {}`, `{"name":"` + strings.Repeat("x", 20<<10) + `"}`} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/agents/connect-token", strings.NewReader(body))
		request.Header.Set("X-Auth-Request-User", "editor")
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest && response.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("body=%q status=%d body=%s", body[:min(len(body), 32)], response.Code, response.Body.String())
		}
	}
}

func TestCloudAgentStoreScopesAgentsByUser(t *testing.T) {
	now := time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC)
	store := newCloudAgentStore(func() time.Time { return now })
	_, _ = store.Upsert(context.Background(), models.CloudAgent{ID: "agent-a", UserID: "user-a", Name: "A", Status: "connected"})
	_, _ = store.Upsert(context.Background(), models.CloudAgent{ID: "agent-b", UserID: "user-b", Name: "B", Status: "connected"})

	agents, err := store.List(context.Background(), "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 || agents[0].ID != "agent-a" {
		t.Fatalf("agents = %#v", agents)
	}
}

func TestCloudAgentChannelAuthenticatesAndTracksConnection(t *testing.T) {
	apiHandler, _, _, _ := reliabilityTestAPI(t)
	apiHandler = withTestRuntime(apiHandler, testCloudRuntimeConfig())
	server := httptest.NewServer(apiHandler.Routes())
	defer server.Close()

	badURL := websocketURL(server.URL, "/api/agents/channel?token=bad")
	if conn, _, err := websocket.DefaultDialer.Dial(badURL, nil); err == nil {
		_ = conn.Close()
		t.Fatal("bad token connected")
	}

	token := apiHandler.cloud.signAgentToken(agentConnectToken{UserID: "user-1", AgentID: "agent-1", Name: "MacBook", Platform: "darwin", ExpiresAt: time.Now().UTC().Add(time.Minute)})
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
	agents, err := apiHandler.cloud.agents.List(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 || agents[0].Status != "connected" {
		t.Fatalf("agents = %#v", agents)
	}
}

func websocketURL(serverURL, path string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http") + path
}
