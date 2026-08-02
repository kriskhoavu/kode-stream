package ai

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const StateInterrupted = "interrupted"

type SessionPlanRef struct {
	ItemID         string `json:"itemId" yaml:"itemId"`
	ItemPath       string `json:"itemPath" yaml:"itemPath"`
	Identifier     string `json:"identifier,omitempty" yaml:"identifier,omitempty"`
	BranchKey      string `json:"branchKey" yaml:"branchKey"`
	ObservedCommit string `json:"observedCommit,omitempty" yaml:"observedCommit,omitempty"`
}

type SessionRecord struct {
	ID              string          `json:"id" yaml:"id"`
	WorkspaceID     string          `json:"workspaceId" yaml:"workspaceId"`
	PlanRef         *SessionPlanRef `json:"planRef,omitempty" yaml:"planRef,omitempty"`
	Provider        string          `json:"provider" yaml:"provider"`
	Intent          string          `json:"intent" yaml:"intent"`
	RequestedBranch string          `json:"requestedBranch" yaml:"requestedBranch"`
	ObservedCommit  string          `json:"observedCommit,omitempty" yaml:"observedCommit,omitempty"`
	IdempotencyKey  string          `json:"-" yaml:"idempotencyKey,omitempty"`
	State           string          `json:"state" yaml:"state"`
	StartedAt       time.Time       `json:"startedAt" yaml:"startedAt"`
	EndedAt         time.Time       `json:"endedAt,omitempty" yaml:"endedAt,omitempty"`
	ExitCode        *int            `json:"exitCode,omitempty" yaml:"exitCode,omitempty"`
	LastKnownAt     time.Time       `json:"lastKnownAt" yaml:"lastKnownAt"`
}

type SessionRecordRepository interface {
	Get(id string) (SessionRecord, bool, error)
	FindByIdempotency(workspaceID, key string) (SessionRecord, bool, error)
	List(workspaceID, branch string) ([]SessionRecord, error)
	Upsert(SessionRecord) (SessionRecord, error)
	Snapshot() ([]SessionRecord, error)
	ReplaceAll([]SessionRecord) error
}

type FileSessionRecordRepository struct {
	path string
	mu   sync.Mutex
}

func NewFileSessionRecordRepository(path string) *FileSessionRecordRepository {
	return &FileSessionRecordRepository{path: path}
}

func (r *FileSessionRecordRepository) Get(id string) (SessionRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records, err := r.load()
	if err != nil {
		return SessionRecord{}, false, err
	}
	for _, record := range records {
		if record.ID == id {
			return record, true, nil
		}
	}
	return SessionRecord{}, false, nil
}

func (r *FileSessionRecordRepository) FindByIdempotency(workspaceID, key string) (SessionRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records, err := r.load()
	if err != nil {
		return SessionRecord{}, false, err
	}
	workspaceID, key = strings.TrimSpace(workspaceID), strings.TrimSpace(key)
	if workspaceID == "" || key == "" {
		return SessionRecord{}, false, nil
	}
	for _, record := range records {
		if record.WorkspaceID == workspaceID && record.IdempotencyKey == key {
			return record, true, nil
		}
	}
	return SessionRecord{}, false, nil
}

func (r *FileSessionRecordRepository) List(workspaceID, branch string) ([]SessionRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records, err := r.load()
	if err != nil {
		return nil, err
	}
	workspaceID, branch = strings.TrimSpace(workspaceID), strings.TrimSpace(branch)
	result := []SessionRecord{}
	for _, record := range records {
		if workspaceID != "" && record.WorkspaceID != workspaceID {
			continue
		}
		if branch != "" && record.RequestedBranch != branch {
			continue
		}
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].StartedAt.Equal(result[j].StartedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].StartedAt.After(result[j].StartedAt)
	})
	return result, nil
}

func (r *FileSessionRecordRepository) Upsert(record SessionRecord) (SessionRecord, error) {
	record = normalizeSessionRecord(record)
	if err := ValidateSessionRecord(record); err != nil {
		return SessionRecord{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	records, err := r.load()
	if err != nil {
		return SessionRecord{}, err
	}
	for i := range records {
		if records[i].ID == record.ID {
			records[i] = record
			return record, r.save(records)
		}
		if record.IdempotencyKey != "" && records[i].WorkspaceID == record.WorkspaceID && records[i].IdempotencyKey == record.IdempotencyKey {
			return SessionRecord{}, fmt.Errorf("idempotency key already belongs to session %q", records[i].ID)
		}
	}
	records = append(records, record)
	return record, r.save(records)
}

func (r *FileSessionRecordRepository) Snapshot() ([]SessionRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records, err := r.load()
	return append([]SessionRecord(nil), records...), err
}

func (r *FileSessionRecordRepository) ReplaceAll(records []SessionRecord) error {
	if records == nil {
		records = []SessionRecord{}
	}
	seen := map[string]bool{}
	idempotency := map[string]string{}
	for i := range records {
		records[i] = normalizeSessionRecord(records[i])
		if err := ValidateSessionRecord(records[i]); err != nil {
			return err
		}
		if seen[records[i].ID] {
			return fmt.Errorf("duplicate session record ID %q", records[i].ID)
		}
		seen[records[i].ID] = true
		if records[i].IdempotencyKey != "" {
			key := records[i].WorkspaceID + "\x00" + records[i].IdempotencyKey
			if prior := idempotency[key]; prior != "" {
				return fmt.Errorf("idempotency key is shared by sessions %q and %q", prior, records[i].ID)
			}
			idempotency[key] = records[i].ID
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.save(records)
}

func ValidateSessionRecord(record SessionRecord) error {
	if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.WorkspaceID) == "" {
		return errors.New("session record identity is incomplete")
	}
	if strings.TrimSpace(record.Provider) == "" || strings.TrimSpace(record.Intent) == "" || strings.TrimSpace(record.RequestedBranch) == "" {
		return errors.New("session record context is incomplete")
	}
	switch record.State {
	case StateStarting, StateRunning, StateExited, StateCancelled, StateFailed, StateInterrupted:
	default:
		return fmt.Errorf("unsupported session record state %q", record.State)
	}
	if record.StartedAt.IsZero() || record.LastKnownAt.IsZero() {
		return errors.New("session record timestamps are required")
	}
	if record.PlanRef != nil {
		if strings.TrimSpace(record.PlanRef.ItemID) == "" || strings.TrimSpace(record.PlanRef.ItemPath) == "" || strings.TrimSpace(record.PlanRef.BranchKey) == "" {
			return errors.New("session plan reference is incomplete")
		}
		if record.PlanRef.BranchKey != record.RequestedBranch {
			return errors.New("session plan branch differs from requested branch")
		}
	}
	return nil
}

func normalizeSessionRecord(record SessionRecord) SessionRecord {
	record.ID = strings.TrimSpace(record.ID)
	record.WorkspaceID = strings.TrimSpace(record.WorkspaceID)
	record.Provider = strings.TrimSpace(record.Provider)
	record.Intent = strings.TrimSpace(record.Intent)
	record.RequestedBranch = strings.TrimSpace(record.RequestedBranch)
	record.ObservedCommit = strings.TrimSpace(record.ObservedCommit)
	record.IdempotencyKey = strings.TrimSpace(record.IdempotencyKey)
	if record.PlanRef != nil {
		plan := *record.PlanRef
		plan.ItemID = strings.TrimSpace(plan.ItemID)
		plan.ItemPath = strings.TrimSpace(plan.ItemPath)
		plan.Identifier = strings.TrimSpace(plan.Identifier)
		plan.BranchKey = strings.TrimSpace(plan.BranchKey)
		plan.ObservedCommit = strings.TrimSpace(plan.ObservedCommit)
		record.PlanRef = &plan
	}
	return record
}

func (r *FileSessionRecordRepository) load() ([]SessionRecord, error) {
	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return []SessionRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	var records []SessionRecord
	if err := yaml.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	if records == nil {
		records = []SessionRecord{}
	}
	for i := range records {
		records[i] = normalizeSessionRecord(records[i])
		if err := ValidateSessionRecord(records[i]); err != nil {
			return nil, err
		}
	}
	return records, nil
}

func (r *FileSessionRecordRepository) save(records []SessionRecord) error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(records)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(r.path), ".ai-session-records-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, r.path)
}
