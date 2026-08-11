package audit

// Audit repository contract tests.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kode-stream/internal/common/models"
)

func TestStoreAppendsAndReadsNewestEventsFirst(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "audit-log.jsonl"))
	times := []time.Time{
		time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 20, 10, 1, 0, 0, time.UTC),
	}
	store.now = func() time.Time {
		value := times[0]
		times = times[1:]
		return value
	}

	first, err := store.Append(models.AuditEvent{Operation: "scan", Status: models.AuditStatusSuccess, Message: "Scanned"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Append(models.AuditEvent{Operation: "save_file", Status: models.AuditStatusSuccess, Message: "Saved"})
	if err != nil {
		t.Fatal(err)
	}

	events, err := store.Recent(1)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || second.ID == "" || first.ID == second.ID {
		t.Fatalf("event IDs were not generated: first=%q second=%q", first.ID, second.ID)
	}
	if len(events) != 1 || events[0].ID != second.ID {
		t.Fatalf("Recent(1) = %#v, want second event", events)
	}
}

func TestQueryFiltersBeforeApplyingLimit(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "audit-log.jsonl"))
	for index := 0; index < 8; index++ {
		workspace := "other"
		if index == 0 || index == 3 {
			workspace = "wanted"
		}
		if _, err := store.Append(models.AuditEvent{OwnerUserID: "owner-a", WorkspaceID: workspace, Operation: "scan", Status: models.AuditStatusSuccess, Message: "event"}); err != nil {
			t.Fatal(err)
		}
	}
	events, err := store.QueryContext(context.Background(), Query{OwnerUserID: "owner-a", WorkspaceID: "wanted", Limit: 2})
	if err != nil || len(events) != 2 {
		t.Fatalf("QueryContext() = %#v, %v; wanted two matches despite newer interleaved events", events, err)
	}
}

func TestCachedQueryKeepsWorkspaceAndOwnerInCacheKey(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "audit-log.jsonl"))
	for index := 0; index < 20; index++ {
		workspace := "other"
		if index < 3 {
			workspace = "wanted"
		}
		if _, err := store.Append(models.AuditEvent{OwnerUserID: "owner", WorkspaceID: workspace, Operation: "event"}); err != nil {
			t.Fatal(err)
		}
	}
	reader := NewCachedEventReader(store, time.Minute, time.Now)
	wanted, err := reader.QueryContext(context.Background(), Query{OwnerUserID: "owner", WorkspaceID: "wanted", Limit: 2})
	if err != nil || len(wanted) != 2 {
		t.Fatalf("wanted=%#v err=%v", wanted, err)
	}
	other, err := reader.QueryContext(context.Background(), Query{OwnerUserID: "owner", WorkspaceID: "other", Limit: 2})
	if err != nil || len(other) != 2 {
		t.Fatalf("other=%#v err=%v", other, err)
	}
	again, err := reader.QueryContext(context.Background(), Query{OwnerUserID: "owner", WorkspaceID: "wanted", Limit: 2})
	if err != nil || len(again) != 2 || reader.Stats().Hits != 1 {
		t.Fatalf("cached=%#v stats=%#v err=%v", again, reader.Stats(), err)
	}
}

func TestStoreSkipsMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit-log.jsonl")
	data := "not-json\n{\"id\":\"valid\",\"time\":\"2026-06-20T10:00:00Z\",\"operation\":\"scan\",\"status\":\"success\",\"message\":\"ok\"}\n{broken\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	events, err := New(path).Recent(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != "valid" {
		t.Fatalf("Recent() = %#v, want valid event only", events)
	}
	if events[0].Paths == nil {
		t.Fatal("Paths must normalize to an empty array")
	}
	if events[0].OwnerUserID != LocalActor || events[0].ActorUserID != LegacyActor {
		t.Fatalf("legacy identity = owner %q actor %q", events[0].OwnerUserID, events[0].ActorUserID)
	}
}

func TestStoreReturnsEmptyWhenFileDoesNotExist(t *testing.T) {
	events, err := New(filepath.Join(t.TempDir(), "missing.jsonl")).Recent(10)
	if err != nil {
		t.Fatal(err)
	}
	if events == nil || len(events) != 0 {
		t.Fatalf("Recent() = %#v, want non-nil empty slice", events)
	}
}
