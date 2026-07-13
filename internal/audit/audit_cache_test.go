package audit

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"kode-stream/internal/common/models"
)

func TestCachedEventReaderUsesTTLAndInvalidation(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "audit.jsonl"))
	now := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	cache := NewCachedEventReader(store, time.Minute, func() time.Time { return now })

	if _, err := store.Append(models.AuditEvent{Operation: "first", Status: models.AuditStatusSuccess, Message: "first"}); err != nil {
		t.Fatal(err)
	}
	first, err := cache.RecentContext(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Operation != "first" {
		t.Fatalf("first read = %#v", first)
	}

	if _, err := store.Append(models.AuditEvent{Operation: "second", Status: models.AuditStatusSuccess, Message: "second"}); err != nil {
		t.Fatal(err)
	}
	cached, err := cache.RecentContext(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cached) != 1 || cached[0].Operation != "first" {
		t.Fatalf("cached read = %#v", cached)
	}

	cache.Invalidate()
	invalidated, err := cache.RecentContext(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(invalidated) != 2 || invalidated[0].Operation != "second" {
		t.Fatalf("invalidated read = %#v", invalidated)
	}

	now = now.Add(2 * time.Minute)
	if _, err := store.Append(models.AuditEvent{Operation: "third", Status: models.AuditStatusSuccess, Message: "third"}); err != nil {
		t.Fatal(err)
	}
	expired, err := cache.RecentContext(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 3 || expired[0].Operation != "third" {
		t.Fatalf("expired read = %#v", expired)
	}

	stats := cache.Stats()
	if stats.Hits != 1 || stats.Misses != 3 || stats.Invalidations != 1 {
		t.Fatalf("stats = %#v", stats)
	}
}
