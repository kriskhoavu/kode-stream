package scanner

import (
	"testing"

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
