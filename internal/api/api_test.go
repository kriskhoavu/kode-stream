package api

import (
	"strings"
	"testing"

	"plan-manager/internal/models"
)

func TestFallbackPlanRoot(t *testing.T) {
	repo := models.RepositoryConfig{PlanDirectories: []string{"plans"}}
	plan := models.PlanDetail{PlanSummary: models.PlanSummary{Service: "api", Ticket: "DI-170"}}

	got := fallbackPlanRoot(repo, plan)
	if got != "plans/api/DI-170" {
		t.Fatalf("fallbackPlanRoot() = %q", got)
	}
}

func TestFallbackPlanRootRequiresPlanDirectory(t *testing.T) {
	plan := models.PlanDetail{PlanSummary: models.PlanSummary{Service: "api", Ticket: "DI-170"}}

	if got := fallbackPlanRoot(models.RepositoryConfig{}, plan); got != "" {
		t.Fatalf("fallbackPlanRoot() = %q, want empty", got)
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

func TestNormalizePlanDetailUsesEmptyCollections(t *testing.T) {
	plan := normalizePlanDetail(models.PlanDetail{})
	if plan.Tags == nil {
		t.Fatal("tags should be an empty slice, got nil")
	}
	if plan.Documents == nil {
		t.Fatal("documents should be an empty slice, got nil")
	}
	if plan.Metadata == nil {
		t.Fatal("metadata should be an empty map, got nil")
	}
}
