package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"plan-manager/internal/itemindex"
	"plan-manager/internal/models"
)

func TestFallbackItemPath(t *testing.T) {
	workspace := models.WorkspaceConfig{Sources: []string{"items"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{Scope: "api", Identifier: "DI-170"}}

	got := fallbackItemPath(workspace, item)
	if got != "items/api/DI-170" {
		t.Fatalf("fallbackItemPath() = %q", got)
	}
}

func TestFallbackItemPathRequiresPlanDirectory(t *testing.T) {
	item := models.ItemDetail{ItemSummary: models.ItemSummary{Scope: "api", Identifier: "DI-170"}}

	if got := fallbackItemPath(models.WorkspaceConfig{}, item); got != "" {
		t.Fatalf("fallbackItemPath() = %q, want empty", got)
	}
}

func TestFirstMarkdownParagraphReturnsFullParagraph(t *testing.T) {
	markdown := "# Title\n\nEvery controller repeats the same permission boilerplate: build an `actionList`, call `isInvalidOfferActions()`, return 403. Controllers also accept `@RequestParam OfferAction action` from the frontend, leaking authorization details into the client contract."

	got := firstMarkdownParagraph(markdown)
	if strings.Contains(got, "...") {
		t.Fatalf("paragraph was truncated: %q", got)
	}
	if !strings.Contains(got, "client contract") {
		t.Fatalf("paragraph did not include the full text: %q", got)
	}
}

func TestNormalizeItemDetailUsesEmptyCollections(t *testing.T) {
	item := normalizeItemDetail(models.ItemDetail{})
	if item.Tags == nil {
		t.Fatal("tags should be an empty slice, got nil")
	}
	if item.Documents == nil {
		t.Fatal("documents should be an empty slice, got nil")
	}
	if item.Metadata == nil {
		t.Fatal("metadata should be an empty map, got nil")
	}
}

func TestValidateGitPathsStaysInsideSources(t *testing.T) {
	workspace := models.WorkspaceConfig{Sources: []string{"items", "docs"}}
	if err := validateGitPaths(workspace, []string{"items/platform/PM-002/README.md", "docs/guide.md"}); err != nil {
		t.Fatalf("expected paths to be valid: %v", err)
	}
}

func TestValidateGitPathsRejectsEscapesAndUnregisteredPaths(t *testing.T) {
	workspace := models.WorkspaceConfig{Sources: []string{"items"}}
	for _, paths := range [][]string{
		{},
		{"../secret.md"},
		{"/tmp/secret.md"},
		{"src/main.go"},
	} {
		if err := validateGitPaths(workspace, paths); err == nil {
			t.Fatalf("expected %#v to be rejected", paths)
		}
	}
}

func TestRoutesListItemsPreservesJSONShape(t *testing.T) {
	dir := t.TempDir()
	idx := itemindex.New(filepath.Join(dir, "item-index.yaml"))
	updatedAt := time.Date(2026, 6, 20, 1, 2, 3, 0, time.UTC)
	if err := idx.ReplaceWorkspace("workspace-1", []models.ItemDetail{{
		ItemSummary: models.ItemSummary{
			ID:             "item-1",
			WorkspaceID:    "workspace-1",
			WorkspaceName:  "Workspace",
			Branch:         "main",
			Scope:          "platform",
			Identifier:     "PM-003",
			Title:          "Architecture",
			Status:         models.StatusDraft,
			UpdatedAt:      updatedAt,
			Description:    "Refactor architecture",
			MetadataSource: "item.yaml",
			ItemPath:       "plans/platform/PM-003",
		},
	}}, nil, updatedAt); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/items?workspaceId=workspace-1&q=architecture", nil)
	res := httptest.NewRecorder()
	New(nil, idx, nil, nil, nil, nil, nil).Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var items []models.ItemSummary
	if err := json.Unmarshal(res.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
	item := items[0]
	if item.ID != "item-1" || item.Identifier != "PM-003" || item.Status != models.StatusDraft || item.MetadataSource != "item.yaml" {
		t.Fatalf("unexpected item response: %+v", item)
	}
	if item.Tags == nil {
		t.Fatal("tags should be normalized to an empty array")
	}
}

func TestRoutesMissingItemReturnsNotFoundJSON(t *testing.T) {
	dir := t.TempDir()
	idx := itemindex.New(filepath.Join(dir, "item-index.yaml"))
	req := httptest.NewRequest(http.MethodGet, "/api/items/missing", nil)
	res := httptest.NewRecorder()

	New(nil, idx, nil, nil, nil, nil, nil).Routes().ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["error"] != "item not found" {
		t.Fatalf("error = %q", payload["error"])
	}
}
