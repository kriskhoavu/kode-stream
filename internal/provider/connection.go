package provider

import (
	"fmt"
	"sync"
)

// ConnectionStore owns opaque, per-user credentials. Callers can resolve an
// integration but never read credential material back through an API response.
type ConnectionStore struct {
	mu     sync.RWMutex
	tokens map[string]string
}

func NewConnectionStore() *ConnectionStore { return &ConnectionStore{tokens: map[string]string{}} }

func (s *ConnectionStore) Save(userID, instanceID, token string) error {
	if userID == "" || instanceID == "" || token == "" {
		return fmt.Errorf("user, provider instance, and token are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[userID+":"+instanceID] = token
	return nil
}

func (s *ConnectionStore) Token(userID, instanceID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, ok := s.tokens[userID+":"+instanceID]
	return token, ok
}

func (s *ConnectionStore) Revoke(userID, instanceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, userID+":"+instanceID)
}
