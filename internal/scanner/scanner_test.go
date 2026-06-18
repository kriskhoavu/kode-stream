package scanner

import (
	"testing"

	"plan-manager/internal/gitadapter"
	"plan-manager/internal/models"
)

func TestNormalizeStatus(t *testing.T) {
	cases := map[string]models.PlanStatus{
		"Ideas":       models.StatusIdeas,
		"draft":       models.StatusDraft,
		"in progress": models.StatusInProgress,
		"in-review":   models.StatusReview,
		"completed":   models.StatusDone,
		"unknown":     models.StatusDraft,
		"":            models.StatusDraft,
	}
	for input, want := range cases {
		if got := NormalizeStatus(input); got != want {
			t.Fatalf("NormalizeStatus(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFallbackDocumentsOrdersKnownPlanFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "# Test\n")
	writeTestFile(t, root, "implementation-plan.md", "# Plan\n")
	writeTestFile(t, root, "scenario/scenario-00-overview.md", "# Scenario\n")
	writeTestFile(t, root, "design/design-01-backend.md", "# Backend\n")

	docs := fallbackDocuments(root)
	if len(docs) != 4 {
		t.Fatalf("expected 4 docs, got %d", len(docs))
	}
	roles := map[string]string{}
	for _, doc := range docs {
		roles[doc.Path] = doc.Role
	}
	if roles["README.md"] != "overview" {
		t.Fatalf("README role = %q", roles["README.md"])
	}
	if roles["implementation-plan.md"] != "implementation" {
		t.Fatalf("implementation role = %q", roles["implementation-plan.md"])
	}
}

func TestFallbackDocumentsReturnsEmptySliceForEmptyPlan(t *testing.T) {
	docs := fallbackDocuments(t.TempDir())
	if docs == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(docs) != 0 {
		t.Fatalf("expected no docs, got %d", len(docs))
	}
}

func TestDocumentCollectionDetection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "a12/guide.md", "# Guide\n")
	entries, err := osReadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	if !shouldScanAsDocumentCollection(root, entries) {
		t.Fatal("expected freestyle markdown root to scan as document collection")
	}
}

func TestStructuredPlanDirectoryDoesNotScanAsDocumentCollection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "api/DI-1/README.md", "# Plan\n")
	entries, err := osReadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	if shouldScanAsDocumentCollection(root, entries) {
		t.Fatal("structured plan root should not scan as one document collection")
	}
}

func TestNestedFreestyleDocsStillScanAsDocumentCollection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "ai/revised/note.md", "# Note\n")
	entries, err := osReadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	if !shouldScanAsDocumentCollection(root, entries) {
		t.Fatal("nested freestyle docs should not look like structured plan folders")
	}
}

func TestRepositorySettingsSplitsFreestyleDocsIntoCards(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/repository-settings.yaml", `version: 1
cards:
  - pathPattern: "{service}/feature/{ticket}"
    fields:
      service: "{service}"
      ticket: "{ticket}"
      title: readme_heading
      status: in_progress
      tags: [docs, "{service}"]
`)
	writeTestFile(t, root, "docs/api/feature/DI-101/README.md", "# API Search\n\nSearch docs.\n")
	writeTestFile(t, root, "docs/webapp/feature/DI-202/README.md", "# Web UI\n\nUI docs.\n")

	data, err := New(gitadapter.New()).Scan(models.RepositoryConfig{
		ID: "repo", Name: "Repo", Path: root, BaselineBranch: "main", PlanDirectories: []string{"docs"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Plans) != 2 {
		t.Fatalf("expected 2 configured cards, got %d (%v)", len(data.Plans), data.Warnings)
	}
	plans := map[string]models.PlanDetail{}
	for _, plan := range data.Plans {
		plans[plan.Ticket] = plan
	}
	api := plans["DI-101"]
	if api.Service != "api" || api.Title != "API Search" || api.Status != models.StatusInProgress || api.MetadataSource != "repository-settings" {
		t.Fatalf("unexpected configured plan: %+v", api.PlanSummary)
	}
	if len(api.Tags) != 2 || api.Tags[0] != "docs" || api.Tags[1] != "api" {
		t.Fatalf("unexpected tags: %#v", api.Tags)
	}
}

func TestInvalidRepositorySettingsFallsBackToDocsCollection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/repository-settings.yaml", `version: 1
cards:
  - pathPattern: "{service}/{ticket}"
    fields:
      service: "{missing}"
      ticket: "{ticket}"
`)
	writeTestFile(t, root, "docs/a12/guide.md", "# Guide\n\nDocs.\n")

	data, err := New(gitadapter.New()).Scan(models.RepositoryConfig{
		ID: "repo", Name: "Repo", Path: root, BaselineBranch: "main", PlanDirectories: []string{"docs"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Plans) != 1 {
		t.Fatalf("expected fallback docs card, got %d", len(data.Plans))
	}
	if data.Plans[0].MetadataSource != "docs" {
		t.Fatalf("expected docs fallback, got %q", data.Plans[0].MetadataSource)
	}
	if len(data.Warnings) == 0 {
		t.Fatal("expected invalid settings warning")
	}
}

func TestRepositorySettingsDoNotOverridePlanYAML(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/repository-settings.yaml", `version: 1
cards:
  - pathPattern: "{service}/feature/{ticket}"
    fields:
      service: "{service}"
      ticket: "{ticket}"
      title: "Configured"
      status: done
      tags: [docs]
`)
	writeTestFile(t, root, "docs/api/feature/DI-101/README.md", "# README Title\n")
	writeTestFile(t, root, "docs/api/feature/DI-101/plan.yaml", `plan:
  ticket: DI-101
  title: YAML Title
  service: backend
  status: review
`)

	data, err := New(gitadapter.New()).Scan(models.RepositoryConfig{
		ID: "repo", Name: "Repo", Path: root, BaselineBranch: "main", PlanDirectories: []string{"docs"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(data.Plans))
	}
	plan := data.Plans[0]
	if plan.MetadataSource != "plan.yaml" || plan.Service != "backend" || plan.Title != "YAML Title" || plan.Status != models.StatusReview {
		t.Fatalf("plan.yaml should win over repository settings: %+v", plan.PlanSummary)
	}
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := root + "/" + rel
	if err := osMkdirAll(path); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(path, content); err != nil {
		t.Fatal(err)
	}
}
