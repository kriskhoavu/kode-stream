package navigation

// Navigation repository contract tests.

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

func TestNavigationSnapshotRestorePreservesIdentityOwnershipAndTimestamps(t *testing.T) {
	dir := t.TempDir()
	store := New(filepath.Join(dir, "filters.yaml"), filepath.Join(dir, "recents.yaml"))
	created := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := created.Add(time.Hour)
	opened := created.Add(2 * time.Hour)
	filter := models.SavedFilter{OwnerUserID: "owner", ID: "stable-filter", Name: "Snapshot", Route: "/workstream", Filters: map[string]any{}, CreatedAt: created, UpdatedAt: updated}
	recent := models.RecentItem{OwnerUserID: "owner", ItemID: "stable-item", WorkspaceID: "workspace", Title: "Snapshot", Route: "/items/stable-item", OpenedAt: opened}
	if err := store.RestoreFilter(filter); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreRecent(recent); err != nil {
		t.Fatal(err)
	}
	filters, _ := store.FiltersForOwner("owner")
	recents, _ := store.RecentsForOwner("owner", 10)
	if len(filters) != 1 || filters[0].ID != filter.ID || !filters[0].CreatedAt.Equal(created) || !filters[0].UpdatedAt.Equal(updated) {
		t.Fatalf("filters = %#v", filters)
	}
	if len(recents) != 1 || recents[0].ItemID != recent.ItemID || !recents[0].OpenedAt.Equal(opened) {
		t.Fatalf("recents = %#v", recents)
	}
}
