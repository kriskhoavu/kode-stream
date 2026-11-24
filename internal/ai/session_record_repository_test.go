package ai

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestFileSessionRecordRepositoryDeletesOneRecord(t *testing.T) {
	repository := NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "ai-session-records.yaml"))
	now := time.Now().UTC()
	for _, id := range []string{"session-1", "session-2"} {
		if _, err := repository.Upsert(SessionRecord{ID: id, WorkspaceID: "workspace-1", Provider: "codex", Intent: "implement", RequestedBranch: "main", State: StateExited, StartedAt: now, LastKnownAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	if deleted, err := repository.Delete("session-1"); err != nil || !deleted {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
	if _, found, err := repository.Get("session-1"); err != nil || found {
		t.Fatalf("deleted record found=%v err=%v", found, err)
	}
	if _, found, err := repository.Get("session-2"); err != nil || !found {
		t.Fatalf("remaining record found=%v err=%v", found, err)
	}
	if deleted, err := repository.Delete("missing"); err != nil || deleted {
		t.Fatalf("missing deleted=%v err=%v", deleted, err)
	}
}

func TestServiceDeletesFinishedSessionButRejectsRunningSession(t *testing.T) {
	repository := NewFileSessionRecordRepository(filepath.Join(t.TempDir(), "ai-session-records.yaml"))
	service := New(NewSettingsRepository(filepath.Join(t.TempDir(), "ai-settings.yaml")))
	service.records = repository
	now := time.Now().UTC()
	finished := SessionRecord{ID: "finished", WorkspaceID: "workspace-1", Provider: "codex", Intent: "implement", RequestedBranch: "main", State: StateExited, StartedAt: now, LastKnownAt: now}
	running := SessionRecord{ID: "running", WorkspaceID: "workspace-1", Provider: "codex", Intent: "implement", RequestedBranch: "main", State: StateRunning, StartedAt: now, LastKnownAt: now}
	for _, record := range []SessionRecord{finished, running} {
		if _, err := repository.Upsert(record); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.DeleteSessionRecord(finished.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteSessionRecord(running.ID); !errors.Is(err, ErrSessionStillRunning) {
		t.Fatalf("err=%v, want ErrSessionStillRunning", err)
	}
	if _, found, err := repository.Get(running.ID); err != nil || !found {
		t.Fatalf("running record found=%v err=%v", found, err)
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

func TestFileSessionRecordRepositoryRejectsInvalidCandidateYAMLWithoutReplacingPriorGeneration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai-session-records.yaml")
	repository := NewFileSessionRecordRepository(path)
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	prior := SessionRecord{ID: "prior", WorkspaceID: "workspace", Provider: "codex", Intent: "implement", RequestedBranch: "main", IdempotencyKey: "prior-key", State: StateExited, StartedAt: now, LastKnownAt: now}
	if _, err := repository.Upsert(prior); err != nil {
		t.Fatal(err)
	}
	previous, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	valid := "- id: one\n  workspaceId: workspace\n  provider: codex\n  intent: implement\n  requestedBranch: main\n  state: exited\n  startedAt: 2026-08-03T12:00:00Z\n  lastKnownAt: 2026-08-03T12:00:00Z\n"
	for name, candidate := range map[string][]byte{
		"malformed":     []byte("- id: [broken\n"),
		"unknown_field": []byte(valid + "  unknown: rejected\n"),
		"duplicate_id":  []byte(valid + valid),
		"duplicate_key": []byte(valid + "- id: two\n  workspaceId: workspace\n  provider: codex\n  intent: implement\n  requestedBranch: main\n  idempotencyKey: shared\n  state: exited\n  startedAt: 2026-08-03T12:00:00Z\n  lastKnownAt: 2026-08-03T12:00:00Z\n"),
	} {
		t.Run(name, func(t *testing.T) {
			if name == "duplicate_key" {
				candidate = []byte(strings.ReplaceAll(string(candidate), "- id: one\n", "- id: one\n  idempotencyKey: shared\n"))
			}
			if _, err := decodeSessionRecords(candidate); err == nil {
				t.Fatal("unsafe candidate was accepted")
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil || string(after) != string(previous) {
				t.Fatalf("prior generation changed: %q err=%v", after, readErr)
			}
			record, found, getErr := repository.Get(prior.ID)
			if getErr != nil || !found || record.ID != prior.ID {
				t.Fatalf("prior generation unreadable: %#v found=%v err=%v", record, found, getErr)
			}
		})
	}
}

func TestFileSessionRecordRepositoryIgnoresStaleLaneIDButRejectsUnsafeFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai-session-records.yaml")
	stale := "- id: s1\n  workspaceId: w1\n  provider: codex\n  intent: implement\n  requestedBranch: main\n  laneId: lane-1\n  state: exited\n  startedAt: 2026-09-11T00:00:00Z\n  lastKnownAt: 2026-09-11T00:00:00Z\n"
	if err := os.WriteFile(path, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}
	records, err := NewFileSessionRecordRepository(path).Snapshot()
	if err != nil {
		t.Fatalf("stale laneId bricked the store: %v", err)
	}
	if len(records) != 1 || records[0].ID != "s1" {
		t.Fatalf("records=%#v", records)
	}
	unsafe := strings.Replace(stale, "laneId: lane-1", "prompt: secret", 1)
	if err := os.WriteFile(path, []byte(unsafe), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileSessionRecordRepository(path).Snapshot(); err == nil {
		t.Fatal("unsafe field was accepted")
	}
}
