package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	appagent "kode-stream/internal/agent"
	"kode-stream/internal/cloudstate"
	"kode-stream/internal/common/models"
)

const agentConnectTokenTTL = 30 * time.Minute

const (
	maxAgentConnectBodyBytes int64 = 16 << 10
	maxAgentNameBytes              = 160
	maxAgentPlatformBytes          = 128
)

type agentConnectToken struct {
	ID                     string    `json:"id"`
	WorkspacePublicationID string    `json:"workspacePublicationId"`
	UserID                 string    `json:"userId"`
	UserEmail              string    `json:"userEmail,omitempty"`
	AgentID                string    `json:"agentId"`
	Name                   string    `json:"name"`
	Platform               string    `json:"platform,omitempty"`
	ExpiresAt              time.Time `json:"expiresAt"`
}

type cloudAgentStore struct {
	mu          sync.RWMutex
	now         func() time.Time
	agents      map[string]map[string]models.CloudAgent
	persistence cloudstate.Repository
	consumed    map[string]time.Time
}

func newCloudAgentStore(now func() time.Time, persistence ...cloudstate.Repository) *cloudAgentStore {
	var repository cloudstate.Repository
	if len(persistence) > 0 {
		repository = persistence[0]
	}
	return &cloudAgentStore{now: now, agents: map[string]map[string]models.CloudAgent{}, persistence: repository, consumed: map[string]time.Time{}}
}

func (s *cloudAgentStore) ConsumeEnrollment(ctx context.Context, token agentConnectToken) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	for id, expiry := range s.consumed {
		if now.After(expiry) {
			delete(s.consumed, id)
		}
	}
	if token.ID == "" {
		// Signed credentials minted by pre-protocol clients did not have a jti.
		// Treat their full stable identity as a one-time identifier so they are
		// never replayable while avoiding an ambiguous compatibility bypass.
		token.ID = stableCloudUserID(token.UserID + ":" + token.AgentID + ":" + token.ExpiresAt.UTC().Format(time.RFC3339Nano))
	}
	if now.After(token.ExpiresAt) {
		return false, nil
	}
	if durable, ok := s.persistence.(cloudstate.EnrollmentTokenRepository); ok {
		return durable.ConsumeEnrollmentToken(ctx, token.ID, token.ExpiresAt)
	}
	if _, used := s.consumed[token.ID]; used {
		return false, nil
	}
	s.consumed[token.ID] = token.ExpiresAt
	return true, nil
}

func (s *cloudAgentStore) Upsert(ctx context.Context, agent models.CloudAgent) (models.CloudAgent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	agent.LastSeenAt = s.now().UTC()
	if s.persistence != nil {
		persisted, err := s.persistence.UpsertAgent(ctx, agent.UserID, agent)
		if err != nil {
			return models.CloudAgent{}, err
		}
		agent = persisted
	}
	if s.agents[agent.UserID] == nil {
		s.agents[agent.UserID] = map[string]models.CloudAgent{}
	}
	s.agents[agent.UserID][agent.ID] = agent
	return agent, nil
}

func (s *cloudAgentStore) List(ctx context.Context, userID string) ([]models.CloudAgent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.persistence != nil {
		persisted, err := s.persistence.ListAgents(ctx, userID)
		if err != nil {
			return nil, err
		}
		for index := range persisted {
			live, ok := s.agents[userID][persisted[index].ID]
			if ok && live.Status == "connected" && s.now().Sub(live.LastSeenAt) <= 2*time.Minute {
				persisted[index] = live
			} else if persisted[index].Status == "connected" {
				persisted[index].Status = "offline"
			}
		}
		return persisted, nil
	}
	owned := s.agents[userID]
	if len(owned) == 0 {
		return []models.CloudAgent{}, nil
	}
	result := make([]models.CloudAgent, 0, len(owned))
	for _, agent := range owned {
		if s.now().Sub(agent.LastSeenAt) > 2*time.Minute && agent.Status == "connected" {
			agent.Status = "stale"
		}
		result = append(result, agent)
	}
	return result, nil
}

func (s *cloudAgentStore) HasConnected(userID, agentID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, ok := s.agents[userID][agentID]
	return ok && agent.Status == "connected" && s.now().Sub(agent.LastSeenAt) <= 2*time.Minute
}

func (a *cloudController) cloudAgentConnectToken(w http.ResponseWriter, r *http.Request) {
	session, ok := cloudSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Cloud session is required")
		return
	}
	var input struct {
		Name     string `json:"name"`
		Platform string `json:"platform"`
	}
	if !decodeLimitedJSON(w, r, &input, maxAgentConnectBodyBytes, true) {
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "Cloud Agent"
	}
	if len(name) > maxAgentNameBytes || len(strings.TrimSpace(input.Platform)) > maxAgentPlatformBytes {
		writeError(w, http.StatusBadRequest, "agent metadata is invalid")
		return
	}
	tokenID, err := newAgentTokenID()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "unable to mint agent credential")
		return
	}
	publicationID, err := newAgentTokenID()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "unable to mint agent credential")
		return
	}
	token := agentConnectToken{
		ID:                     tokenID,
		WorkspacePublicationID: publicationID,
		UserID:                 session.User.ID,
		UserEmail:              session.User.Email,
		AgentID:                stableCloudUserID(session.User.ID + ":" + name),
		Name:                   name,
		Platform:               strings.TrimSpace(input.Platform),
		ExpiresAt:              time.Now().UTC().Add(agentConnectTokenTTL),
	}
	signed := a.signAgentToken(token)
	writeJSON(w, http.StatusOK, map[string]any{
		"token":     signed,
		"expiresAt": token.ExpiresAt,
		"deepLink":  "kodestream://connect?token=" + signed,
	})
}

func (a *cloudController) consumeWorkspacePublication(ctx context.Context, token agentConnectToken) (bool, error) {
	if token.WorkspacePublicationID == "" || time.Now().UTC().After(token.ExpiresAt) {
		return false, nil
	}
	if durable, ok := a.agents.persistence.(cloudstate.EnrollmentTokenRepository); ok {
		return durable.ConsumeEnrollmentToken(ctx, token.WorkspacePublicationID, token.ExpiresAt)
	}
	// The in-memory store uses the same atomic set as channel enrollment but a
	// distinct random ID, so consuming publication cannot affect connection.
	return a.agents.ConsumeEnrollment(ctx, agentConnectToken{ID: token.WorkspacePublicationID, ExpiresAt: token.ExpiresAt})
}

func (a *cloudController) cloudAgents(w http.ResponseWriter, r *http.Request) {
	session, ok := cloudSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Cloud session is required")
		return
	}
	agents, err := a.agents.List(r.Context(), session.User.ID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Cloud agent persistence is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func (a *cloudController) cloudAgentChannel(w http.ResponseWriter, r *http.Request) {
	token, ok := a.verifyAgentToken(r.URL.Query().Get("token"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid agent connect token")
		return
	}
	consumed, consumeErr := a.agents.ConsumeEnrollment(r.Context(), token)
	if consumeErr != nil {
		writeError(w, http.StatusServiceUnavailable, "Cloud agent persistence is unavailable")
		return
	}
	if !consumed {
		writeError(w, http.StatusUnauthorized, "agent connect token has already been used")
		return
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == "" }}
	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	connection.SetReadLimit(appagent.MaxFrameBytes)
	agent, err := a.agents.Upsert(r.Context(), models.CloudAgent{ID: token.AgentID, UserID: token.UserID, Name: token.Name, Platform: token.Platform, Status: "connected"})
	if err != nil {
		_ = connection.WriteJSON(appagent.Frame{Type: "error", Error: "Cloud agent persistence is unavailable"})
		return
	}
	_ = connection.WriteJSON(appagent.Frame{Type: appagent.FrameConnected, Agent: agent})
	for {
		var frame appagent.Frame
		if err := connection.ReadJSON(&frame); err != nil {
			if _, persistErr := a.agents.Upsert(context.Background(), models.CloudAgent{ID: token.AgentID, UserID: token.UserID, Name: token.Name, Platform: token.Platform, Status: "offline"}); persistErr != nil {
				log.Printf("cloud_agent_persistence_failure operation=%q owner_user_id=%q agent_id=%q error=%q", "disconnect", token.UserID, token.AgentID, persistErr)
			}
			return
		}
		if err := appagent.ValidateFrame(frame, token.AgentID, token.UserID); err != nil || frame.Type != appagent.FrameHeartbeat {
			_ = connection.WriteJSON(appagent.Frame{Type: "error", Error: "invalid agent frame"})
			return
		}
		if frame.Type == appagent.FrameHeartbeat {
			agent, err = a.agents.Upsert(r.Context(), models.CloudAgent{ID: token.AgentID, UserID: token.UserID, Name: token.Name, Platform: token.Platform, Status: "connected"})
			if err != nil {
				_ = connection.WriteJSON(appagent.Frame{Type: "error", Error: "Cloud agent persistence is unavailable"})
				return
			}
			_ = connection.WriteJSON(appagent.Frame{Type: appagent.FrameHeartbeatAck, Agent: agent})
		}
	}
}

func newAgentTokenID() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func (a *cloudController) signAgentToken(token agentConnectToken) string {
	data, _ := json.Marshal(token)
	payload := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, []byte(a.runtimeConfig.CookieSecret))
	mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + signature
}

func (a *cloudController) verifyAgentToken(value string) (agentConnectToken, bool) {
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
