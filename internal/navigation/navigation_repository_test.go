package navigation

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"kode-stream/internal/common/models"
)

func TestSavedFiltersCreateListUpdateAndDelete(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "filters.yaml"), filepath.Join(t.TempDir(), "recents.yaml"))
	times := []time.Time{time.Unix(1, 0), time.Unix(2, 0)}
	store.now = func() time.Time { value := times[0]; times = times[1:]; return value }
	first, err := store.SaveFilter(models.SavedFilter{Name: "Drafts", Route: "/workstream", Filters: map[string]any{"status": "draft"}})
	if err != nil || first.ID == "" {
		t.Fatalf("SaveFilter() = %#v, %v", first, err)
	}
	first.Name = "My drafts"
	updated, err := store.SaveFilter(first)
	if err != nil || updated.CreatedAt != first.CreatedAt || !updated.UpdatedAt.After(updated.CreatedAt) {
		t.Fatalf("updated = %#v, %v", updated, err)
	}
	filters, err := store.Filters()
	if err != nil || len(filters) != 1 || filters[0].Name != "My drafts" {
		t.Fatalf("Filters() = %#v, %v", filters, err)
	}
	deleted, err := store.DeleteFilter(first.ID)
	if err != nil || !deleted {
		t.Fatalf("DeleteFilter() = %v, %v", deleted, err)
	}
}

func TestOwnedNavigationIsIsolatedAndRecentRetentionIsPerOwner(t *testing.T) {
	dir := t.TempDir()
	store := New(filepath.Join(dir, "filters.yaml"), filepath.Join(dir, "recents.yaml"))
	clock := int64(0)
	store.now = func() time.Time { clock++; return time.Unix(clock, 0) }
	filter, err := store.SaveFilterForOwner("owner-a", models.SavedFilter{Name: "A", Route: "/workstream"})
	if err != nil {
		t.Fatal(err)
	}
	if deleted, err := store.DeleteFilterForOwner("owner-b", filter.ID); err != nil || deleted {
		t.Fatalf("cross-owner delete = %v, %v", deleted, err)
	}
	if filters, err := store.FiltersForOwner("owner-b"); err != nil || len(filters) != 0 {
		t.Fatalf("owner-b filters = %#v, %v", filters, err)
	}
	for index := 0; index < 51; index++ {
		if err := store.RecordRecentForOwner("owner-a", models.RecentItem{ItemID: fmt.Sprintf("a-%02d", index), Title: "A"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.RecordRecentForOwner("owner-b", models.RecentItem{ItemID: "b-1", Title: "B"}); err != nil {
		t.Fatal(err)
	}
	ownerA, err := store.RecentsForOwner("owner-a", 0)
	if err != nil || len(ownerA) != 50 || ownerA[len(ownerA)-1].ItemID != "a-01" {
		t.Fatalf("owner-a recents = %d %#v, %v", len(ownerA), ownerA, err)
	}
	ownerB, err := store.RecentsForOwner("owner-b", 0)
	if err != nil || len(ownerB) != 1 || ownerB[0].ItemID != "b-1" {
		t.Fatalf("owner-b recents = %#v, %v", ownerB, err)
	}
}

func TestRecentItemsDeduplicateAndOrderNewestFirst(t *testing.T) {
	dir := t.TempDir()
	store := New(filepath.Join(dir, "filters.yaml"), filepath.Join(dir, "recents.yaml"))
	times := []time.Time{time.Unix(1, 0), time.Unix(2, 0), time.Unix(3, 0)}
	store.now = func() time.Time { value := times[0]; times = times[1:]; return value }
	for _, item := range []models.RecentItem{{ItemID: "one", Title: "One"}, {ItemID: "two", Title: "Two"}, {ItemID: "one", Title: "One again"}} {
		if err := store.RecordRecent(item); err != nil {
			t.Fatal(err)
		}
	}
	recents, err := store.Recents(10)
	if err != nil || len(recents) != 2 || recents[0].ItemID != "one" || recents[0].Title != "One again" {
		t.Fatalf("Recents() = %#v, %v", recents, err)
	}
}

func TestMissingNavigationFilesReturnEmptyCollections(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "filters.yaml"), filepath.Join(t.TempDir(), "recents.yaml"))
	filters, filterErr := store.Filters()
	recents, recentErr := store.Recents(10)
	if filterErr != nil || recentErr != nil || filters == nil || recents == nil || len(filters) != 0 || len(recents) != 0 {
		t.Fatalf("filters=%#v recents=%#v errors=%v,%v", filters, recents, filterErr, recentErr)
	}
}

func TestAtomicNavigationWriteFailureLeavesPreviousSnapshotReadable(t *testing.T) {
	dir := t.TempDir()
	store := New(filepath.Join(dir, "filters.yaml"), filepath.Join(dir, "recents.yaml"))
	created, err := store.SaveFilterForOwner("owner", models.SavedFilter{Name: "Before", Route: "/workstream"})
	if err != nil {
		t.Fatal(err)
	}
	store.write = func(string, any) error { return errors.New("injected rename failure") }
	created.Name = "After"
	if _, err := store.SaveFilterForOwner("owner", created); err == nil {
		t.Fatal("expected injected write failure")
	}
	store.write = writeYAML
	filters, err := store.FiltersForOwner("owner")
	if err != nil || len(filters) != 1 || filters[0].Name != "Before" {
		t.Fatalf("persisted filters = %#v, %v", filters, err)
	}
}
