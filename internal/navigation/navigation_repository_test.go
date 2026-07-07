package navigation

// Navigation repository contract tests.

import (
	"path/filepath"
	"testing"
	"time"

	"plan-manager/internal/common/models"
)

func TestSavedFiltersCreateListUpdateAndDelete(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "filters.yaml"), filepath.Join(t.TempDir(), "recents.yaml"))
	times := []time.Time{time.Unix(1, 0), time.Unix(2, 0)}
	store.now = func() time.Time { value := times[0]; times = times[1:]; return value }
	first, err := store.SaveFilter(models.SavedFilter{Name: "Drafts", Route: "/workspace", Filters: map[string]any{"status": "draft"}})
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
