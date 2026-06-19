package search

import (
	"testing"

	"plan-manager/internal/itemindex"
	"plan-manager/internal/models"
)

type itemStub struct{ items []models.ItemSummary }

func (s itemStub) Query(query itemindex.Query) ([]models.ItemSummary, error) {
	items := make([]models.ItemSummary, 0)
	for _, item := range s.items {
		if query.WorkspaceID == "" || item.WorkspaceID == query.WorkspaceID {
			items = append(items, item)
		}
	}
	return items, nil
}

func TestSearchRanksExactIdentifierBeforeOtherMatches(t *testing.T) {
	service := New(itemStub{items: []models.ItemSummary{
		{ID: "1", Identifier: "PM-005", Title: "Search", WorkspaceID: "w1", WorkspaceName: "Plan Manager"},
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
