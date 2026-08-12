package search

// Search service contract tests.

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/item/index"
)

type itemStub struct {
	items []models.ItemSummary
	query *itemindex.Query
}

type visitingItemStub struct{ items []models.ItemSummary }

func (s visitingItemStub) Query(query itemindex.Query) ([]models.ItemSummary, error) {
	return itemStub{items: s.items}.Query(query)
}
func (s visitingItemStub) VisitContext(ctx context.Context, query itemindex.Query, visit func(models.ItemSummary) bool) error {
	for _, item := range s.items {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !visit(item) {
			return nil
		}
	}
	return nil
}

func (s itemStub) Query(query itemindex.Query) ([]models.ItemSummary, error) {
	if s.query != nil {
		*s.query = query
	}
	items := make([]models.ItemSummary, 0)
	for _, item := range s.items {
		if query.WorkspaceID == "" || item.WorkspaceID == query.WorkspaceID {
			items = append(items, item)
		}
	}
	return items, nil
}

func TestSearchUsesOperationalItemQuery(t *testing.T) {
	var query itemindex.Query
	service := New(itemStub{items: []models.ItemSummary{{ID: "checkout", Identifier: "PM-038"}}, query: &query})
	if _, err := service.Search(models.SearchQuery{Text: "PM-038"}); err != nil {
		t.Fatal(err)
	}
	if query.IncludeSnapshots {
		t.Fatal("global search opted into reviewed snapshots")
	}
}

func TestSearchDoesNotReturnReviewedSnapshotRows(t *testing.T) {
	idx := itemindex.New(filepath.Join(t.TempDir(), "items.yaml"))
	now := time.Now().UTC()
	if err := idx.ReplaceWorkspaceBranch("workspace", "main", []models.ItemDetail{{
		ItemSummary: models.ItemSummary{ID: "checkout", WorkspaceID: "workspace", Branch: "main", Identifier: "PM-038", SourceMode: "working_tree"},
	}}, models.BranchScanMetadata{SourceMode: "working_tree", ScannedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := idx.ReplaceWorkspaceBranch("workspace", "feature", []models.ItemDetail{{
		ItemSummary: models.ItemSummary{ID: "review", WorkspaceID: "workspace", Branch: "feature", Identifier: "PM-038-REVIEW", SourceMode: "snapshot"},
	}}, models.BranchScanMetadata{SourceMode: "snapshot", ScannedAt: now}); err != nil {
		t.Fatal(err)
	}

	results, err := New(idx).Search(models.SearchQuery{Text: "PM-038"})
	if err != nil || len(results) != 1 || results[0].ID != "checkout" {
		t.Fatalf("results = %#v, err = %v", results, err)
	}
}

func TestSearchRanksExactIdentifierBeforeOtherMatches(t *testing.T) {
	service := New(itemStub{items: []models.ItemSummary{
		{ID: "1", Identifier: "PM-005", Title: "Search", WorkspaceID: "w1", WorkspaceName: "Kode Stream"},
		{ID: "2", Identifier: "PM-005-NOTES", Title: "Notes", WorkspaceID: "w1"},
		{ID: "3", Identifier: "OTHER", Title: "PM-005 follow up", WorkspaceID: "w1"},
	}})

	results, err := service.Search(models.SearchQuery{Text: "PM-005"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || results[0].ID != "1" || results[0].Score != 100 || results[1].Score != 90 || results[2].Score != 80 {
		t.Fatalf("results = %#v", results)
	}
	if results[0].Route != "/items/1" {
		t.Fatalf("route = %q", results[0].Route)
	}
}

func TestSearchFiltersWorkspaceAndLimitsResults(t *testing.T) {
	service := New(itemStub{items: []models.ItemSummary{
		{ID: "1", Identifier: "PM-001", WorkspaceID: "w1"},
		{ID: "2", Identifier: "PM-002", WorkspaceID: "w1"},
		{ID: "3", Identifier: "PM-003", WorkspaceID: "w2"},
	}})
	results, err := service.Search(models.SearchQuery{Text: "PM-", WorkspaceID: "w1", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].WorkspaceID != "w1" {
		t.Fatalf("results = %#v", results)
	}
}

func TestSearchReturnsEmptyForBlankQueryOrExcludedType(t *testing.T) {
	service := New(itemStub{items: []models.ItemSummary{{ID: "1", Title: "Search"}}})
	for _, query := range []models.SearchQuery{{}, {Text: "Search", Types: []string{"workspace"}}} {
		results, err := service.Search(query)
		if err != nil || results == nil || len(results) != 0 {
			t.Fatalf("Search(%#v) = %#v, %v", query, results, err)
		}
	}
}

func TestSearchRejectsOversizedQueryAndCanceledContext(t *testing.T) {
	service := New(itemStub{items: []models.ItemSummary{{ID: "1", Title: "Search"}}})
	if _, err := service.Search(models.SearchQuery{Text: strings.Repeat("x", 201)}); err == nil {
		t.Fatal("expected oversized query error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.SearchContext(ctx, models.SearchQuery{Text: "search"}); err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestSearchKeepsLaterBetterMatchesWhenCandidateBudgetIsFull(t *testing.T) {
	items := make([]models.ItemSummary, 0, maxSearchCandidates+1)
	for index := 0; index < maxSearchCandidates; index++ {
		items = append(items, models.ItemSummary{ID: fmt.Sprintf("weak-%d", index), Identifier: fmt.Sprintf("PM-SEARCH-%d", index), Title: fmt.Sprintf("weak %04d", index)})
	}
	items = append(items, models.ItemSummary{ID: "exact", Identifier: "PM-SEARCH", Title: "Exact"})
	results, err := New(visitingItemStub{items: items}).SearchContext(context.Background(), models.SearchQuery{Text: "PM-SEARCH", Limit: 1})
	if err != nil || len(results) != 1 || results[0].ID != "exact" {
		t.Fatalf("results = %#v, err = %v", results, err)
	}
}
