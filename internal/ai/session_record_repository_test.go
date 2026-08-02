package ai

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFileSessionRecordRepositoryPersistsSafeMetadata(t *testing.T) {
	repository := NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "ai-session-records.yaml"))
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	record := SessionRecord{
		ID: "session-1", WorkspaceID: "workspace-1", Provider: "codex", Intent: "implement", RequestedBranch: "feature/PM-037",
		PlanRef: &SessionPlanRef{ItemID: "item-1", ItemPath: "plans/platform/PM-037", BranchKey: "feature/PM-037"},
		State:   StateRunning, StartedAt: now, LastKnownAt: now,
	}
	saved, err := repository.Upsert(record)
	if err != nil || saved.ID != record.ID {
		t.Fatalf("saved = %#v err=%v", saved, err)
	}
	records, err := repository.List("workspace-1", "feature/PM-037")
	if err != nil || len(records) != 1 || records[0].State != StateRunning {
		t.Fatalf("records = %#v err=%v", records, err)
	}
	record.State = StateInterrupted
	record.LastKnownAt = now.Add(time.Minute)
	if _, err := repository.Upsert(record); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := repository.Get(record.ID)
	if err != nil || !ok || loaded.State != StateInterrupted {
		t.Fatalf("loaded = %#v ok=%v err=%v", loaded, ok, err)
	}
}

func TestFileSessionRecordRepositoryRejectsUnsafeOrIncompleteShape(t *testing.T) {
	repository := NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "ai-session-records.yaml"))
	_, err := repository.Upsert(SessionRecord{ID: "session-1", WorkspaceID: "workspace-1", Provider: "codex", Intent: "implement", RequestedBranch: "main", State: StateRunning})
	if err == nil {
		t.Fatal("expected timestamp validation error")
	}
}

func TestFileSessionRecordRepositoryRejectsDuplicateIdempotencyKey(t *testing.T) {
	repository := NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "ai-session-records.yaml"))
	now := time.Now().UTC()
	base := SessionRecord{ID: "session-1", WorkspaceID: "workspace-1", Provider: "codex", Intent: "implement", RequestedBranch: "main", IdempotencyKey: "request-1", State: StateStarting, StartedAt: now, LastKnownAt: now}
	if _, err := repository.Upsert(base); err != nil {
		t.Fatal(err)
	}
	base.ID = "session-2"
	if _, err := repository.Upsert(base); err == nil {
		t.Fatal("expected duplicate idempotency key to be rejected")
	}
}
