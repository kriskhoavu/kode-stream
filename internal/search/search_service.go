package search

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/item/index"
)

const (
	maxSearchQueryRunes = 200
	maxSearchCandidates = 1000
)

type itemReader interface {
	Query(itemindex.Query) ([]models.ItemSummary, error)
}
type contextItemVisitor interface {
	VisitContext(context.Context, itemindex.Query, func(models.ItemSummary) bool) error
}

type SearchService struct{ items itemReader }

func New(items itemReader) *SearchService { return &SearchService{items: items} }

func (s *SearchService) Search(query models.SearchQuery) ([]models.SearchResult, error) {
	return s.SearchContext(context.Background(), query)
}

func (s *SearchService) SearchContext(ctx context.Context, query models.SearchQuery) ([]models.SearchResult, error) {
	text := strings.ToLower(strings.TrimSpace(query.Text))
	if text == "" || !includesType(query.Types, "item") {
		return []models.SearchResult{}, nil
	}
	if len([]rune(text)) > maxSearchQueryRunes {
		return nil, fmt.Errorf("search query must contain at most %d characters", maxSearchQueryRunes)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	results := make([]models.SearchResult, 0, maxSearchCandidates)
	consider := func(item models.ItemSummary) bool {
		score, context := rank(item, text)
		if score == 0 {
			return true
		}
		results = append(results, models.SearchResult{
			ID: item.ID, Type: "item", Title: firstNonEmpty(item.Title, item.Identifier),
			Subtitle: strings.Trim(strings.Join([]string{item.WorkspaceName, item.Scope, item.Branch}, " · "), " ·"),
			Context:  context, WorkspaceID: item.WorkspaceID, ItemID: item.ID,
			Route: "/items/" + url.PathEscape(item.ID), Score: score,
		})
		if len(results) > maxSearchCandidates {
			sort.SliceStable(results, func(i, j int) bool { return searchResultLess(results[i], results[j]) })
			results = results[:maxSearchCandidates]
		}
		return true
	}
	itemQuery := itemindex.Query{WorkspaceID: query.WorkspaceID}
	if visitor, ok := s.items.(contextItemVisitor); ok {
		if err := visitor.VisitContext(ctx, itemQuery, consider); err != nil {
			return nil, err
		}
	} else {
		items, err := s.items.Query(itemQuery)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !consider(item) {
				break
			}
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return searchResultLess(results[i], results[j]) })
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

func searchResultLess(left, right models.SearchResult) bool {
	if left.Score != right.Score {
		return left.Score > right.Score
	}
	return strings.ToLower(left.Title) < strings.ToLower(right.Title)
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
