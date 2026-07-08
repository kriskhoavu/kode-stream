package search

import (
	"net/url"
	"sort"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/item/index"
)

type itemReader interface {
	Query(itemindex.Query) ([]models.ItemSummary, error)
}

type SearchService struct{ items itemReader }

func New(items itemReader) *SearchService { return &SearchService{items: items} }

func (s *SearchService) Search(query models.SearchQuery) ([]models.SearchResult, error) {
	text := strings.ToLower(strings.TrimSpace(query.Text))
	if text == "" || !includesType(query.Types, "item") {
		return []models.SearchResult{}, nil
	}
	items, err := s.items.Query(itemindex.Query{WorkspaceID: query.WorkspaceID})
	if err != nil {
		return nil, err
	}
	results := make([]models.SearchResult, 0)
	for _, item := range items {
		score, context := rank(item, text)
		if score == 0 {
			continue
		}
		results = append(results, models.SearchResult{
			ID: item.ID, Type: "item", Title: firstNonEmpty(item.Title, item.Identifier),
			Subtitle: strings.Trim(strings.Join([]string{item.WorkspaceName, item.Scope, item.Branch}, " · "), " ·"),
			Context:  context, WorkspaceID: item.WorkspaceID, ItemID: item.ID,
			Route: "/items/" + url.PathEscape(item.ID), Score: score,
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return strings.ToLower(results[i].Title) < strings.ToLower(results[j].Title)
	})
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func rank(item models.ItemSummary, query string) (int, string) {
	identifier := strings.ToLower(item.Identifier)
	title := strings.ToLower(item.Title)
	switch {
	case identifier == query || strings.ToLower(item.ID) == query:
		return 100, item.Identifier
	case strings.HasPrefix(identifier, query):
		return 90, item.Identifier
	case containsWords(title, query):
		return 80, item.Title
	case containsAny(query, item.Scope, item.WorkspaceName):
		return 60, firstNonEmpty(item.Scope, item.WorkspaceName)
	case containsAny(query, item.Description, item.Author, item.Owner, strings.Join(item.Tags, " ")):
		return 40, firstNonEmpty(item.Description, strings.Join(item.Tags, ", "), item.Author, item.Owner)
	case containsAny(query, item.Branch):
		return 30, item.Branch
	default:
		return 0, ""
	}
}

func containsWords(value, query string) bool {
	if strings.Contains(value, query) {
		return true
	}
	for _, word := range strings.Fields(query) {
		if !strings.Contains(value, word) {
			return false
		}
	}
	return query != ""
}

func containsAny(query string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func includesType(types []string, expected string) bool {
	if len(types) == 0 {
		return true
	}
	for _, value := range types {
		if strings.EqualFold(strings.TrimSpace(value), expected) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
