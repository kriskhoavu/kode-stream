package scanner

import (
	"testing"

	"plan-manager/internal/gitadapter"
	"plan-manager/internal/models"
)

func TestNormalizeStatus(t *testing.T) {
	cases := map[string]models.ItemStatus{
		"unsorted":    models.StatusUnsorted,
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
	writeTestFile(t, root, "implementation-item.md", "# Item\n")
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
	if roles["implementation-item.md"] != "implementation" {
		t.Fatalf("implementation role = %q", roles["implementation-item.md"])
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
	writeTestFile(t, root, "api/DI-1/README.md", "# Item\n")
	entries, err := osReadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	if shouldScanAsDocumentCollection(root, entries) {
		t.Fatal("structured item root should not scan as one document collection")
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
		t.Fatal("nested freestyle docs should not look like structured item folders")
	}
}

func TestSourceSettingsModeDetectsStructuredRoot(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "api/DI-1/README.md", "# Item\n")

	if got := SourceSettingsMode(root); got != "structured" {
		t.Fatalf("SourceSettingsMode() = %q, want structured", got)
	}
}

func TestSourceSettingsModeDetectsUnstructuredRoot(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "notes/guide.md", "# Guide\n")

	if got := SourceSettingsMode(root); got != "unstructured" {
		t.Fatalf("SourceSettingsMode() = %q, want unstructured", got)
	}
}

func TestSourceStructureSettingsSplitsFreestyleDocsIntoCards(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/workspace-settings.yaml", `version: 1
cards:
  - pathPattern: "{scope}/feature/{identifier}"
    fields:
      scope: "{scope}"
      identifier: "{identifier}"
      title: readme_heading
      status: in_progress
      tags: [docs, "{scope}"]
`)
	writeTestFile(t, root, "docs/api/feature/DI-101/README.md", "# API Search\n\nSearch docs.\n")
	writeTestFile(t, root, "docs/webapp/feature/DI-202/README.md", "# Web UI\n\nUI docs.\n")

	data, err := New(gitadapter.New()).Scan(models.WorkspaceConfig{
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: []string{"docs"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Items) != 2 {
		t.Fatalf("expected 2 configured cards, got %d (%v)", len(data.Items), data.Warnings)
	}
	items := map[string]models.ItemDetail{}
	for _, item := range data.Items {
		items[item.Identifier] = item
	}
	api := items["DI-101"]
	if api.Scope != "api" || api.Title != "API Search" || api.Status != models.StatusInProgress || api.MetadataSource != "workspace-settings" {
		t.Fatalf("unexpected configured item: %+v", api.ItemSummary)
	}
	if len(api.Tags) != 2 || api.Tags[0] != "docs" || api.Tags[1] != "api" {
		t.Fatalf("unexpected tags: %#v", api.Tags)
	}
}

func TestInvalidSourceStructureSettingsFallsBackToDocsCollection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/workspace-settings.yaml", `version: 1
cards:
  - pathPattern: "{scope}/{identifier}"
    fields:
      scope: "{missing}"
      identifier: "{identifier}"
`)
	writeTestFile(t, root, "docs/a12/guide.md", "# Guide\n\nDocs.\n")

	data, err := New(gitadapter.New()).Scan(models.WorkspaceConfig{
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: []string{"docs"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Items) != 1 {
		t.Fatalf("expected fallback docs card, got %d", len(data.Items))
	}
	if data.Items[0].MetadataSource != "docs" {
		t.Fatalf("expected docs fallback, got %q", data.Items[0].MetadataSource)
	}
	if data.Items[0].Status != models.StatusUnsorted {
		t.Fatalf("expected unsorted docs fallback, got %q", data.Items[0].Status)
	}
	if len(data.Warnings) == 0 {
		t.Fatal("expected invalid settings warning")
	}
}

func TestSourceStructureSettingsDoNotOverridePlanYAML(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/workspace-settings.yaml", `version: 1
cards:
  - pathPattern: "{scope}/feature/{identifier}"
    fields:
      scope: "{scope}"
      identifier: "{identifier}"
      title: "Configured"
      status: done
      tags: [docs]
`)
	writeTestFile(t, root, "docs/api/feature/DI-101/README.md", "# README Title\n")
	writeTestFile(t, root, "docs/api/feature/DI-101/item.yaml", `item:
  identifier: DI-101
  title: YAML Title
  scope: backend
  status: review
`)

	data, err := New(gitadapter.New()).Scan(models.WorkspaceConfig{
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: []string{"docs"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(data.Items))
	}
	item := data.Items[0]
	if item.MetadataSource != "item.yaml" || item.Scope != "backend" || item.Title != "YAML Title" || item.Status != models.StatusReview {
		t.Fatalf("item.yaml should win over workspace settings: %+v", item.ItemSummary)
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
