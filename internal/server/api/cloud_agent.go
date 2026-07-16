package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	appagent "kode-stream/internal/agent"
	"kode-stream/internal/common/models"
)

const agentConnectTokenTTL = 30 * time.Minute

type agentConnectToken struct {
	UserID    string    `json:"userId"`
	UserEmail string    `json:"userEmail,omitempty"`
	AgentID   string    `json:"agentId"`
	Name      string    `json:"name"`
	Platform  string    `json:"platform,omitempty"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type cloudAgentStore struct {
	mu     sync.RWMutex
	now    func() time.Time
	agents map[string]map[string]models.CloudAgent
}

func newCloudAgentStore(now func() time.Time) *cloudAgentStore {
	return &cloudAgentStore{now: now, agents: map[string]map[string]models.CloudAgent{}}
}

func (s *cloudAgentStore) Upsert(agent models.CloudAgent) models.CloudAgent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.agents[agent.UserID] == nil {
		s.agents[agent.UserID] = map[string]models.CloudAgent{}
	}
	agent.LastSeenAt = s.now().UTC()
	s.agents[agent.UserID][agent.ID] = agent
	return agent
}

func (s *cloudAgentStore) List(userID string) []models.CloudAgent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	owned := s.agents[userID]
	if len(owned) == 0 {
		return []models.CloudAgent{}
	}
	result := make([]models.CloudAgent, 0, len(owned))
	for _, agent := range owned {
		if s.now().Sub(agent.LastSeenAt) > 2*time.Minute && agent.Status == "connected" {
			agent.Status = "stale"
		}
		result = append(result, agent)
	}
	return result
}

func (s *cloudAgentStore) HasConnected(userID, agentID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, ok := s.agents[userID][agentID]
	return ok && agent.Status == "connected" && s.now().Sub(agent.LastSeenAt) <= 2*time.Minute
}

func (a *API) cloudAgentConnectToken(w http.ResponseWriter, r *http.Request) {
	session, ok := cloudSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Cloud session is required")
		return
	}
	var input struct {
		Name     string `json:"name"`
		Platform string `json:"platform"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "Cloud Agent"
	}
	token := agentConnectToken{
		UserID:    session.User.ID,
		UserEmail: session.User.Email,
		AgentID:   stableCloudUserID(session.User.ID + ":" + name),
		Name:      name,
		Platform:  strings.TrimSpace(input.Platform),
		ExpiresAt: time.Now().UTC().Add(agentConnectTokenTTL),
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":     a.signAgentToken(token),
		"expiresAt": token.ExpiresAt,
		"deepLink":  "kodestream://connect?token=" + a.signAgentToken(token),
	})
}

func (a *API) cloudAgents(w http.ResponseWriter, r *http.Request) {
	session, ok := cloudSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Cloud session is required")
		return
	}
	writeJSON(w, http.StatusOK, a.agentStore.List(session.User.ID))
}

func (a *API) cloudAgentChannel(w http.ResponseWriter, r *http.Request) {
	token, ok := a.verifyAgentToken(r.URL.Query().Get("token"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid agent connect token")
		return
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	agent := a.agentStore.Upsert(models.CloudAgent{ID: token.AgentID, UserID: token.UserID, Name: token.Name, Platform: token.Platform, Status: "connected"})
	_ = connection.WriteJSON(appagent.Frame{Type: appagent.FrameConnected, Agent: agent})
	for {
		var frame appagent.Frame
		if err := connection.ReadJSON(&frame); err != nil {
			a.agentStore.Upsert(models.CloudAgent{ID: token.AgentID, UserID: token.UserID, Name: token.Name, Platform: token.Platform, Status: "offline"})
			return
		}
		if frame.Type == appagent.FrameHeartbeat {
			agent = a.agentStore.Upsert(models.CloudAgent{ID: token.AgentID, UserID: token.UserID, Name: token.Name, Platform: token.Platform, Status: "connected"})
			_ = connection.WriteJSON(appagent.Frame{Type: appagent.FrameHeartbeatAck, Agent: agent})
		}
	}
}

func (a *API) signAgentToken(token agentConnectToken) string {
	data, _ := json.Marshal(token)
	payload := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, []byte(a.runtimeConfig.CookieSecret))
	mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + signature
}

func (a *API) verifyAgentToken(value string) (agentConnectToken, bool) {
	payload, signature, ok := strings.Cut(value, ".")
	if !ok || a.runtimeConfig.CookieSecret == "" {
		return agentConnectToken{}, false
	}
	mac := hmac.New(sha256.New, []byte(a.runtimeConfig.CookieSecret))
	mac.Write([]byte(payload))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return agentConnectToken{}, false
	}
	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return agentConnectToken{}, false
	}
	var token agentConnectToken
	if err := json.Unmarshal(data, &token); err != nil || time.Now().UTC().After(token.ExpiresAt) {
		return agentConnectToken{}, false
	}
	return token, true
}
