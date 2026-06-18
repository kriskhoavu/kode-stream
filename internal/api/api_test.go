package api

import (
	"strings"
	"testing"

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
