package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"plan-manager/internal/models"
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
