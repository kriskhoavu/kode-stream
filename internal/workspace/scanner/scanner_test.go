package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
)

func TestScanContextHonorsCancellationAndFileBudget(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "plans/platform/PM-1/README.md", strings.Repeat("x", 64))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := New(gitadapter.New())
	if _, err := service.ScanContext(ctx, models.WorkspaceConfig{ID: "w", Path: root, BaselineBranch: "main", Sources: models.SourceList("plans")}, DefaultScanBudget()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error=%v", err)
	}
	budget := DefaultScanBudget()
	budget.MaxFileBytes = 8
	if _, err := service.ScanContext(context.Background(), models.WorkspaceConfig{ID: "w", Path: root, BaselineBranch: "main", Sources: models.SourceList("plans")}, budget); !errors.Is(err, ErrScanLimit) {
		t.Fatalf("budget error=%v", err)
	}
}

func TestNormalizeStatus(t *testing.T) {
	cases := map[string]models.ItemStatus{
		"unsorted":    models.StatusUnsorted,
		"Ideas":       models.StatusDraft,
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

func TestFallbackDocumentsTreatsSolutionFolderAsDesign(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "# Test\n")
	writeTestFile(t, root, "implementation-plan.md", "# Implementation\n")
	writeTestFile(t, root, "scenario/scenario-00-overview.md", "# Scenario\n")
	writeTestFile(t, root, "solution/solution-01-backend.md", "# Backend Design\n")
	writeTestFile(t, root, "solution/solution-02-frontend.md", "# Frontend Design\n")
	writeTestFile(t, root, "design/design-01-uiux.md", "# UI/UX Design\n")

	reader := NewFilesystemSourceReader(root)
	docs := fallbackDocuments(reader, "")
	if len(docs) != 6 {
		t.Fatalf("expected 6 docs, got %d", len(docs))
	}
	paths := []string{"README.md", "scenario/scenario-00-overview.md", "design/design-01-uiux.md", "solution/solution-01-backend.md", "solution/solution-02-frontend.md", "implementation-plan.md"}
	for i, want := range paths {
		if docs[i].Path != want {
			t.Fatalf("document %d = %q, want %q (order %#v)", i, docs[i].Path, want, docs)
		}
	}
	for _, doc := range docs[2:5] {
		if doc.Role != "design" {
			t.Fatalf("%s role = %q, want design", doc.Path, doc.Role)
		}
	}
	if docs[3].Label != "Backend Design" || docs[3].Track != "backend" {
		t.Fatalf("unexpected backend solution metadata: %#v", docs[3])
	}
	if docs[4].Label != "Frontend Design" || docs[4].Track != "frontend" {
		t.Fatalf("unexpected frontend solution metadata: %#v", docs[4])
	}
	if docs[2].Label != "Uiux" || docs[2].Track != "" {
		t.Fatalf("unexpected design folder metadata: %#v", docs[2])
	}
	if docs[0].Role != "overview" || docs[5].Role != "implementation" || docs[1].Label != "Scenario Overview" {
		t.Fatalf("unexpected overview, scenario or implementation metadata: %#v", docs)
	}
}

func TestFallbackDocumentsReturnsEmptySliceForEmptyPlan(t *testing.T) {
	reader := NewFilesystemSourceReader(t.TempDir())
	docs := fallbackDocuments(reader, "")
	if docs == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(docs) != 0 {
		t.Fatalf("expected no docs, got %d", len(docs))
	}
}

func TestDocumentCollectionDetection(t *testing.T) {
	for _, tc := range []struct {
		name       string
		file       string
		collection bool
	}{
		{"freestyle markdown root", "a12/guide.md", true},
		{"structured item root", "api/DI-1/README.md", false},
		{"nested freestyle docs", "ai/revised/note.md", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, tc.file, "# Page\n")
			reader := NewFilesystemSourceReader(root)
			entries, err := reader.ReadDir("")
			if err != nil {
				t.Fatal(err)
			}
			if got := shouldScanAsDocumentCollection(reader, "", entries); got != tc.collection {
				t.Fatalf("shouldScanAsDocumentCollection = %v, want %v", got, tc.collection)
			}
		})
	}
}

func TestSourceSettingsModeDetectsTheRootLayout(t *testing.T) {
	for file, want := range map[string]string{"api/DI-1/README.md": "structured", "notes/guide.md": "unstructured"} {
		root := t.TempDir()
		writeTestFile(t, root, file, "# Page\n")
		if got := SourceSettingsMode(root); got != want {
			t.Fatalf("SourceSettingsMode() with %s = %q, want %q", file, got, want)
		}
	}
}

func TestSourceStructureSettingsSplitsFreestyleDocsIntoCards(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/workspace-settings.yaml", `version: 1
cards:
  - pathPattern: "{folder}/feature/{item}"
    fields:
      source: docs
      item: "{item}"
      title: readme_heading
      status: in_progress
      tags: [docs, "{folder}"]
`)
	writeTestFile(t, root, "docs/api/feature/DI-101/README.md", "# API Search\n\nSearch docs.\n")
	writeTestFile(t, root, "docs/webapp/feature/DI-202/README.md", "# Web UI\n\nUI docs.\n")

	data, err := New(gitadapter.New()).Scan(models.WorkspaceConfig{
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: models.SourceList("docs"),
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
	if api.Scope != "docs" || api.Title != "API Search" || api.Status != models.StatusInProgress || api.MetadataSource != "workspace-settings" {
		t.Fatalf("unexpected configured item: %+v", api.ItemSummary)
	}
	if len(api.Tags) != 2 || api.Tags[0] != "docs" || api.Tags[1] != "api" {
		t.Fatalf("unexpected tags: %#v", api.Tags)
	}
}

func TestTheRepositorysSettingsFileGivesEachSourceTheRulesThatNameIt(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".kode-stream/workspace-settings.yaml", `version: 1
cards:
  - pathPattern: "{item}"
    fields: { source: notes, item: "{item}", title: readme_heading, status: in_progress }
  - pathPattern: "{folder}/feature/{item}"
    fields: { source: specs, item: "{item}", title: readme_heading, status: review }
`)
	writeTestFile(t, root, "notes/workspace-settings.yaml", "version: 1\ncards:\n  - pathPattern: \"{item}\"\n    fields: { source: notes, item: \"{item}\", status: done }\n")
	writeTestFile(t, root, "notes/offers/README.md", "# Offers\n")
	writeTestFile(t, root, "specs/api/feature/DI-101/README.md", "# API Search\n")
	writeTestFile(t, root, "docs/guide.md", "# Guide\n")

	data, err := New(gitadapter.New()).Scan(models.WorkspaceConfig{
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: models.SourceList("notes", "specs", "docs"),
	})
	if err != nil {
		t.Fatal(err)
	}
	items := map[string]models.ItemDetail{}
	for _, item := range data.Items {
		items[item.Identifier] = item
	}
	if offers := items["offers"]; offers.Status != models.StatusInProgress || offers.MetadataSource != "workspace-settings" {
		t.Fatalf("notes must take its rule from .kode-stream/workspace-settings.yaml: %+v (%v)", offers.ItemSummary, data.Warnings)
	}
	if api := items["DI-101"]; api.Status != models.StatusReview || api.Scope != "specs" {
		t.Fatalf("specs must take its own rule from the same file: %+v (%v)", api.ItemSummary, data.Warnings)
	}
	if docs, found := items["docs"]; !found || docs.MetadataSource == "workspace-settings" {
		t.Fatalf("a source that no rule names keeps its default layout: %v", data.Items)
	}
}

func TestSavingAndResettingASourceKeepsTheOtherSourcesRules(t *testing.T) {
	root := t.TempDir()
	repository := filepath.Join(root, ".kode-stream", "workspace-settings.yaml")
	rule := func(source, status string) models.SourceStructureCard {
		return models.SourceStructureCard{PathPattern: "{item}", Fields: models.SourceStructureFields{Source: source, Item: "{item}", Title: "readme_heading", Status: status}}
	}
	if path, exists := SourceSettingsPath(root, "notes"); exists || path != repository {
		t.Fatalf("a source without settings saves to %q, exists = %v", path, exists)
	}
	for _, source := range []string{"notes", "specs"} {
		if err := WriteSourceStructureSettings(root, source, models.SourceStructureSettings{Version: 1, Cards: []models.SourceStructureCard{rule(source, "draft")}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteSourceStructureSettings(root, "notes", models.SourceStructureSettings{Version: 1, Cards: []models.SourceStructureCard{rule("notes", "done")}}); err != nil {
		t.Fatal(err)
	}
	notes, _, _ := ReadSourceStructureSettings(root, "notes")
	specs, _, _ := ReadSourceStructureSettings(root, "specs")
	if len(notes.Cards) != 1 || notes.Cards[0].Fields.Status != "done" || len(specs.Cards) != 1 || specs.Cards[0].Fields.Status != "draft" {
		t.Fatalf("notes = %#v specs = %#v", notes.Cards, specs.Cards)
	}
	if err := WriteSourceStructureSettings(root, "notes", models.SourceStructureSettings{Version: 1, Cards: []models.SourceStructureCard{rule("handbook", "done")}}); err == nil || !strings.Contains(err.Error(), "fields.source must be notes") {
		t.Fatalf("a rule that does not name its source must be refused, err = %v", err)
	}

	if err := RemoveSourceStructureSettings(root, "notes"); err != nil {
		t.Fatal(err)
	}
	if _, exists := SourceSettingsPath(root, "notes"); exists {
		t.Fatal("reset left the notes rule")
	}
	if specs, has, _ := ReadSourceStructureSettings(root, "specs"); !has || len(specs.Cards) != 1 {
		t.Fatalf("reset of notes removed specs: %#v", specs.Cards)
	}
	if err := RemoveSourceStructureSettings(root, "specs"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(repository); !os.IsNotExist(err) {
		t.Fatalf("the file must go with its last rule: %v", err)
	}
}

func TestSavingSettingsRefusesALinkedSettingsFolder(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".kode-stream")); err != nil {
		t.Fatal(err)
	}
	settings := BuiltInStructuredSettings()
	settings.Cards[0].Fields.Source = "wiki"
	if err := WriteSourceStructureSettings(root, "wiki", settings); err == nil {
		t.Fatal("a write through a linked .kode-stream folder must be refused")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("the write left the workspace: %v", entries)
	}
}

func TestSourceStructureSettingsSupportNestedItemIdentifierTemplate(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/workspace-settings.yaml", `version: 1
cards:
  - pathPattern: "{folder}/{item}"
    fields:
      source: docs
      item: "{folder}/{item}"
      title: readme_heading
      status: draft
      tags: [docs]
`)
	writeTestFile(t, root, "docs/a12/prod/README.md", "# Production Profile\n")
	writeTestFile(t, root, "docs/a12/test/README.md", "# Test Profile\n")

	data, err := New(gitadapter.New()).Scan(models.WorkspaceConfig{
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: models.SourceList("docs"),
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
	if _, ok := items["a12/prod"]; !ok {
		t.Fatalf("expected item identifier a12/prod, got %#v", data.Items)
	}
	if _, ok := items["a12/test"]; !ok {
		t.Fatalf("expected item identifier a12/test, got %#v", data.Items)
	}
}

func TestSourceStructureProposalsPreviewRealPaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/api/feature/DI-101/README.md", "# DI-101: API Search\n\nSearch docs.\n")
	writeTestFile(t, root, "docs/web/feature/DI-202/README.md", "# Web UI\n\nUI docs.\n")
	reader := NewFilesystemSourceReader(root)

	proposals, preview := SourceStructureProposals(reader, "docs", DefaultSourceStructureSettings())

	if len(proposals) == 0 {
		t.Fatalf("expected proposal options, got %#v", proposals)
	}
	first := proposals[0]
	if first.ID != "actual-folder-feature-item" || first.Confidence != "high" {
		t.Fatalf("unexpected first proposal: %#v", first)
	}
	if len(first.Preview) != 2 {
		t.Fatalf("expected 2 preview rows, got %#v", first.Preview)
	}
	if first.Preview[0].Path != "docs/api/feature/DI-101" || first.Preview[0].Source != "docs" || first.Preview[0].Item != "DI-101" || first.Preview[0].Title != "API Search" {
		t.Fatalf("unexpected preview row: %#v", first.Preview[0])
	}
	if len(first.Preview[0].Tags) != 1 || first.Preview[0].Tags[0] != "docs" {
		t.Fatalf("unexpected preview tags: %#v", first.Preview[0].Tags)
	}
	if len(preview) != 2 {
		t.Fatalf("expected current settings preview, got %#v", preview)
	}
}

func TestSourceStructureProposalsDerivePatternFromActualSource(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "plans/platform/PM-014/README.md", "# PM-014: Visual Source Structure Proposals\n")
	writeTestFile(t, root, "plans/platform/PM-015/README.md", "# PM-015: Real Suggestions\n")
	reader := NewFilesystemSourceReader(root)

	proposals, _ := SourceStructureProposals(reader, "plans", DefaultSourceStructureSettings())

	if len(proposals) == 0 {
		t.Fatal("expected proposals from actual source directories")
	}
	if got := proposals[0].Card.PathPattern; got != "{folder}/{item}" {
		t.Fatalf("expected actual plans pattern first, got %q from %#v", got, proposals)
	}
	if got := proposals[0].Preview[0].Path; got != "plans/platform/PM-014" {
		t.Fatalf("expected real preview path, got %q", got)
	}
	for _, proposal := range proposals {
		if proposal.Card.PathPattern == "{folder}/feature/{item}" {
			t.Fatalf("unexpected blind feature proposal for plans source: %#v", proposal)
		}
	}
}

func TestSourceStructureProposalsExplainTopLevelFolderWithRootMarkdown(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/README.md", "# Docs\n")
	writeTestFile(t, root, "docs/overview.md", "# Overview\n")
	writeTestFile(t, root, "docs/a12/intro.md", "# A12\n")
	reader := NewFilesystemSourceReader(root)

	proposals, preview := SourceStructureProposals(reader, "docs", DefaultSourceStructureSettings())

	if len(proposals) == 0 {
		t.Fatal("expected proposals from actual source directories")
	}
	first := proposals[0]
	if first.Label != "Item folders" || first.Card.PathPattern != "{item}" {
		t.Fatalf("unexpected top-level proposal: %#v", first)
	}
	if len(first.Preview) != 1 || first.Preview[0].Path != "docs/a12" {
		t.Fatalf("expected a12 preview, got %#v", first.Preview)
	}
	if !strings.Contains(first.Summary, "docs/a12") || !strings.Contains(first.Summary, "2 root Markdown files") {
		t.Fatalf("expected concrete summary with root markdown note, got %q", first.Summary)
	}
	if len(preview) != 0 {
		t.Fatalf("default feature preview should not match mixed docs, got %#v", preview)
	}
}

func TestRemoveSourceStructureSettingsDeletesCurrentFile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/workspace-settings.yaml", "version: 1\ncards: []\n")

	path, exists := SourceSettingsPath(root, "docs")
	if !exists || path != filepath.Join(root, "docs", "workspace-settings.yaml") {
		t.Fatalf("settings path = %q, exists = %v", path, exists)
	}
	if err := RemoveSourceStructureSettings(root, "docs"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "workspace-settings.yaml")); !os.IsNotExist(err) {
		t.Fatalf("workspace-settings.yaml still exists or stat failed unexpectedly: %v", err)
	}
}

func TestDefaultSourceStructureSettingsUsesWikiWhenNoSourceRootIsGiven(t *testing.T) {
	settings := DefaultSourceStructureSettings()
	if settings.Cards[0].Fields.Source != "wiki" || len(settings.Cards[0].Fields.Tags) != 1 || settings.Cards[0].Fields.Tags[0] != "wiki" {
		t.Fatalf("expected Wiki defaults, got %#v", settings.Cards[0].Fields)
	}
}

func TestInvalidSourceStructureSettingsFallsBackToDocumentationCollection(t *testing.T) {
	for _, source := range []string{"docs", "wiki"} {
		t.Run(source, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, source+"/workspace-settings.yaml", `version: 1
cards:
  - pathPattern: "{scope}/{identifier}"
    fields:
      scope: "{missing}"
      identifier: "{identifier}"
`)
			writeTestFile(t, root, source+"/a12/guide.md", "# Guide\n\nDocumentation.\n")

			data, err := New(gitadapter.New()).Scan(models.WorkspaceConfig{
				ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main",
				Sources: []models.WorkspaceSource{{Path: source, ShownIn: []models.SourceSurface{models.SourceSurfaceWorkstream}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(data.Items) != 1 {
				t.Fatalf("expected fallback %s card, got %d", source, len(data.Items))
			}
			if data.Items[0].MetadataSource != source {
				t.Fatalf("expected %s fallback, got %q", source, data.Items[0].MetadataSource)
			}
			if data.Items[0].Status != models.StatusUnsorted {
				t.Fatalf("expected unsorted %s fallback, got %q", source, data.Items[0].Status)
			}
			if len(data.Warnings) == 0 {
				t.Fatal("expected invalid settings warning")
			}
		})
	}
}

func TestSourceStructureSettingsDoNotOverridePlanYAML(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/workspace-settings.yaml", `version: 1
cards:
  - pathPattern: "{scope}/feature/{identifier}"
    fields:
      source: "{scope}"
      item: "{identifier}"
      title: "Configured"
      status: done
      tags: [docs]
`)
	writeTestFile(t, root, "docs/api/feature/DI-101/README.md", "# README Title\n")
	writeTestFile(t, root, "docs/api/feature/DI-101/plan.yaml", `plan:
  identifier: DI-101
  title: YAML Title
  scope: backend
  status: review
`)

	data, err := New(gitadapter.New()).Scan(models.WorkspaceConfig{
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: models.SourceList("docs"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(data.Items))
	}
	item := data.Items[0]
	if item.MetadataSource != "plan.yaml" || item.Scope != "backend" || item.Title != "YAML Title" || item.Status != models.StatusReview {
		t.Fatalf("plan.yaml should win over workspace settings: %+v", item.ItemSummary)
	}
}

func TestStructuredSourceScanCharacterization(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "plans/platform/PM-003/README.md", "# Technical Architecture Refactoring\n\nImprove internals.\n")
	writeTestFile(t, root, "plans/platform/PM-003/design/design-01-architecture.md", "# Architecture\n")
	writeTestFile(t, root, "plans/platform/PM-003/scenario/scenario-00-overview.md", "# Scenario\n")
	writeTestFile(t, root, "plans/platform/PM-003/plan.yaml", `schemaVersion: 1
plan:
  ticket: PM-003
  title: YAML Architecture Title
  service: platform
  status: review
  tags:
    - architecture
    - refactoring
documents:
  - id: overview
    role: overview
    path: README.md
    label: Overview
`)

	data, err := New(gitadapter.New()).Scan(models.WorkspaceConfig{
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: models.SourceList("plans"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Items) != 1 {
		t.Fatalf("expected 1 structured item, got %d (%v)", len(data.Items), data.Warnings)
	}
	item := data.Items[0]
	if item.Scope != "platform" || item.Identifier != "PM-003" || item.Title != "YAML Architecture Title" {
		t.Fatalf("unexpected structured item identity: %+v", item.ItemSummary)
	}
	if item.Status != models.StatusReview || item.MetadataSource != "plan.yaml" {
		t.Fatalf("unexpected structured item metadata: %+v", item.ItemSummary)
	}
	if len(item.Documents) != 3 || item.Documents[0].Path != "README.md" {
		t.Fatalf("unexpected documents: %#v", item.Documents)
	}
	if item.Counts.Files != 3 {
		t.Fatalf("file count = %d, want 3", item.Counts.Files)
	}
}

func TestMinimalPlanYAMLInfersIdentityTitleAndDocuments(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "plans/api/DI-170/README.md", "# DI-170: Custom Assortment Level 2\n\nOverview.\n")
	writeTestFile(t, root, "plans/api/DI-170/scenario/scenario-00-overview.md", "# Scenario\n")
	writeTestFile(t, root, "plans/api/DI-170/design/design-01-backend.md", "# Backend\n")
	writeTestFile(t, root, "plans/api/DI-170/implementation-plan.md", "# Implementation\n")
	writeTestFile(t, root, "plans/api/DI-170/plan.yaml", `plan:
  status: done
  tags:
    - custom-assortment
    - offer-detail
`)

	data, err := New(gitadapter.New()).Scan(models.WorkspaceConfig{
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: models.SourceList("plans"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(data.Items))
	}
	item := data.Items[0]
	if item.Scope != "api" || item.Identifier != "DI-170" || item.Title != "Custom Assortment Level 2" || item.Status != models.StatusDone {
		t.Fatalf("unexpected inferred plan: %+v", item.ItemSummary)
	}
	if len(item.Tags) != 2 || item.Tags[0] != "custom-assortment" || item.Tags[1] != "offer-detail" {
		t.Fatalf("unexpected tags: %#v", item.Tags)
	}
	if len(item.Documents) != 4 || item.Documents[3].Role != "implementation" {
		t.Fatalf("unexpected inferred documents: %#v", item.Documents)
	}
	if item.Documents[1].Label != "Scenario Overview" || item.Documents[2].Label != "Backend Design" {
		t.Fatalf("unexpected inferred labels: %#v", item.Documents)
	}
}

func TestMinimalPlanYAMLAppliesSparseDocumentOverrides(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "plans/api/DI-170/README.md", "# DI-170: Example\n")
	writeTestFile(t, root, "plans/api/DI-170/design/design-01-auto-assign.md", "# Design: Auto Assign\n")
	writeTestFile(t, root, "plans/api/DI-170/scenario/scenario-00-overview.md", "# Scenario\n")
	writeTestFile(t, root, "plans/api/DI-170/plan.yaml", `plan:
  status: done
documents:
  - path: design/design-01-auto-assign.md
    track: backend
    label: Auto-Assign Enricher
`)

	data, err := New(gitadapter.New()).Scan(models.WorkspaceConfig{
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: models.SourceList("plans"),
	})
	if err != nil {
		t.Fatal(err)
	}
	docs := data.Items[0].Documents
	if len(docs) != 3 {
		t.Fatalf("sparse override should retain inferred documents: %#v", docs)
	}
	if docs[2].Track != "backend" || docs[2].Label != "Auto-Assign Enricher" {
		t.Fatalf("sparse override was not applied: %#v", docs[2])
	}
}

func TestScanWithRequestMatchesFilesystemAndGitTreeForCommittedContent(t *testing.T) {
	root := newReaderGitRepo(t)
	writeReaderGitFile(t, root, "plans/platform/PM-013/README.md", "# PM-013: Snapshot Materialization\n\nOverview.\n")
	writeReaderGitFile(t, root, "plans/platform/PM-013/plan.yaml", "plan:\n  status: review\n  tags: [branch]\n")
	writeReaderGitFile(t, root, "plans/platform/PM-013/design/design-01-backend.md", "# Backend\n")
	readerGitCommit(t, root, "add plan")

	workspace := models.WorkspaceConfig{ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: models.SourceList("plans")}
	git := gitadapter.New()
	scan := New(git)
	fsData, err := scan.ScanWithRequest(ScanRequest{
		Workspace:  workspace,
		Branch:     "main",
		SourceMode: "working_tree",
		Editable:   true,
		Reader:     NewFilesystemSourceReader(root),
	})
	if err != nil {
		t.Fatal(err)
	}
	ref, commit, err := git.ResolveBranch(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	gitData, err := scan.ScanWithRequest(ScanRequest{
		Workspace:  workspace,
		Branch:     "main",
		BranchRef:  ref,
		Commit:     commit,
		SourceMode: "snapshot",
		Editable:   false,
		Reader:     NewGitTreeSourceReader(root, ref, git),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fsData.Items) != 1 || len(gitData.Items) != 1 {
		t.Fatalf("items fs=%d git=%d warnings=%v/%v", len(fsData.Items), len(gitData.Items), fsData.Warnings, gitData.Warnings)
	}
	fsItem := fsData.Items[0]
	gitItem := gitData.Items[0]
	if fsItem.Identifier != gitItem.Identifier || fsItem.Title != gitItem.Title || fsItem.Status != gitItem.Status || fsItem.Counts.Files != gitItem.Counts.Files {
		t.Fatalf("scan mismatch fs=%+v git=%+v", fsItem.ItemSummary, gitItem.ItemSummary)
	}
	if gitItem.BranchRef != ref || gitItem.Commit != commit || gitItem.SourceMode != "snapshot" || gitItem.Editable {
		t.Fatalf("snapshot metadata not stamped: %+v", gitItem.ItemSummary)
	}
}

func TestScanWithRequestDocsSnapshotDoesNotWarnOnMissingPlanYAML(t *testing.T) {
	root := newReaderGitRepo(t)
	writeReaderGitFile(t, root, "docs/a12/a12-in-discovery.md", "# A12 in Discovery\n")
	readerGitCommit(t, root, "add docs")

	workspace := models.WorkspaceConfig{ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: models.SourceList("docs")}
	git := gitadapter.New()
	ref, commit, err := git.ResolveBranch(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	data, err := New(git).ScanWithRequest(ScanRequest{
		Workspace:  workspace,
		Branch:     "main",
		BranchRef:  ref,
		Commit:     commit,
		SourceMode: "snapshot",
		Editable:   false,
		Reader:     NewGitTreeSourceReader(root, ref, git),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Warnings) != 0 {
		t.Fatalf("warnings = %#v", data.Warnings)
	}
	if len(data.Items) != 1 {
		t.Fatalf("items = %#v", data.Items)
	}
	if len(data.Items[0].Warnings) != 0 {
		t.Fatalf("item warnings = %#v", data.Items[0].Warnings)
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

func TestScanReadsOnlySourcesShownInWorkstream(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "plans/a12/PM-1/plan.md", "# Plan\n")
	writeTestFile(t, root, "wiki/index.md", "# Wiki\n")
	writeTestFile(t, root, "wiki/guide.md", "# Guide\n\nDocumentation.\n")

	data, err := New(gitadapter.New()).Scan(models.WorkspaceConfig{
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: models.SourceList("plans", "wiki"),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range data.Items {
		if strings.HasPrefix(item.ItemPath, "wiki/") {
			t.Fatalf("wiki page %q reached the board", item.ItemPath)
		}
	}
	if len(data.Items) == 0 {
		t.Fatal("expected the plan on the board")
	}
}
