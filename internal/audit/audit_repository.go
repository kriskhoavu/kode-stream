package audit

// This package owns the append-only audit repository.

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"kode-stream/internal/common/models"
)

type AuditRepository struct {
	mu   sync.Mutex
	path string
	now  func() time.Time
}

type Repository interface {
	Append(models.AuditEvent) (models.AuditEvent, error)
	Recent(int) ([]models.AuditEvent, error)
	RecentContext(context.Context, int) ([]models.AuditEvent, error)
	QueryContext(context.Context, Query) ([]models.AuditEvent, error)
}

type Query struct {
	OwnerUserID string
	WorkspaceID string
	Limit       int
}

func (s *Store) RecentContext(ctx context.Context, limit int) ([]models.AuditEvent, error) {
	return s.QueryContext(ctx, Query{Limit: limit})
}

func (s *Store) QueryContext(ctx context.Context, query Query) ([]models.AuditEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	events, err := s.Recent(0)
	if err != nil {
		return nil, err
	}
	filtered := make([]models.AuditEvent, 0, len(events))
	for _, event := range events {
		if query.OwnerUserID != "" && event.OwnerUserID != query.OwnerUserID {
			continue
		}
		if query.WorkspaceID != "" && event.WorkspaceID != query.WorkspaceID {
			continue
		}
		filtered = append(filtered, event)
		if query.Limit > 0 && len(filtered) == query.Limit {
			break
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return filtered, nil
}

type Store = AuditRepository

func New(path string) *AuditRepository {
	return &AuditRepository{path: path, now: time.Now}
}

func (s *Store) Append(event models.AuditEvent) (models.AuditEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if event.ID == "" {
		event.ID = newID()
	}
	if event.Time.IsZero() {
		event.Time = s.now().UTC()
	}
	if event.Paths == nil {
		event.Paths = []string{}
	}
	data, err := json.Marshal(event)
	if err != nil {
		return models.AuditEvent{}, err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return models.AuditEvent{}, err
	}
	file, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return models.AuditEvent{}, err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return models.AuditEvent{}, err
	}
	return event, file.Sync()
}

func (s *Store) Recent(limit int) ([]models.AuditEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []models.AuditEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	events := make([]models.AuditEvent, 0)
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		var event models.AuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		if event.Paths == nil {
			event.Paths = []string{}
		}
		event = NormalizeLegacyEvent(event)
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for left, right := 0, len(events)-1; left < right; left, right = left+1, right-1 {
		events[left], events[right] = events[right], events[left]
	}
	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

func NormalizeLegacyEvent(event models.AuditEvent) models.AuditEvent {
	if event.OwnerUserID == "" {
		event.OwnerUserID = LocalActor
	}
	if event.ActorUserID == "" {
		event.ActorUserID = LegacyActor
	}
	return event
}

func newID() string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return time.Now().UTC().Format("20060102150405.000000000")
}
