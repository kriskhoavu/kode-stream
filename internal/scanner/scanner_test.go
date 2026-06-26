package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"plan-manager/internal/gitadapter"
	"plan-manager/internal/models"
)

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

func TestFallbackDocumentsOrdersKnownPlanFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "# Test\n")
	writeTestFile(t, root, "implementation-plan.md", "# Implementation\n")
	writeTestFile(t, root, "scenario/scenario-00-overview.md", "# Scenario\n")
	writeTestFile(t, root, "design/design-01-backend.md", "# Backend\n")

	reader := NewFilesystemSourceReader(root)
	docs := fallbackDocuments(reader, "")
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
	if docs[0].Path != "README.md" || docs[1].Path != "scenario/scenario-00-overview.md" || docs[2].Path != "design/design-01-backend.md" || docs[3].Path != "implementation-plan.md" {
		t.Fatalf("unexpected document order: %#v", docs)
	}
	if docs[1].Label != "Scenario Overview" || docs[2].Label != "Backend Design" || docs[2].Track != "backend" {
		t.Fatalf("unexpected inferred metadata: %#v", docs)
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
	root := t.TempDir()
	writeTestFile(t, root, "a12/guide.md", "# Guide\n")
	reader := NewFilesystemSourceReader(root)
	entries, err := reader.ReadDir("")
	if err != nil {
		t.Fatal(err)
	}

	if !shouldScanAsDocumentCollection(reader, "", entries) {
		t.Fatal("expected freestyle markdown root to scan as document collection")
	}
}

func TestStructuredPlanDirectoryDoesNotScanAsDocumentCollection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "api/DI-1/README.md", "# Item\n")
	reader := NewFilesystemSourceReader(root)
	entries, err := reader.ReadDir("")
	if err != nil {
		t.Fatal(err)
	}

	if shouldScanAsDocumentCollection(reader, "", entries) {
		t.Fatal("structured item root should not scan as one document collection")
	}
}

func TestNestedFreestyleDocsStillScanAsDocumentCollection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "ai/revised/note.md", "# Note\n")
	reader := NewFilesystemSourceReader(root)
	entries, err := reader.ReadDir("")
	if err != nil {
		t.Fatal(err)
	}

	if !shouldScanAsDocumentCollection(reader, "", entries) {
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
	if api.Scope != "docs" || api.Title != "API Search" || api.Status != models.StatusInProgress || api.MetadataSource != "workspace-settings" {
		t.Fatalf("unexpected configured item: %+v", api.ItemSummary)
	}
	if len(api.Tags) != 2 || api.Tags[0] != "docs" || api.Tags[1] != "api" {
		t.Fatalf("unexpected tags: %#v", api.Tags)
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

func TestRemoveSourceStructureSettingsDeletesCurrentAndLegacyFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/workspace-settings.yaml", "version: 1\ncards: []\n")
	writeTestFile(t, root, "docs/repository-settings.yaml", "version: 1\ncards: []\n")

	if err := RemoveSourceStructureSettings(filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "workspace-settings.yaml")); !os.IsNotExist(err) {
		t.Fatalf("workspace-settings.yaml still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "repository-settings.yaml")); !os.IsNotExist(err) {
		t.Fatalf("repository-settings.yaml still exists or stat failed unexpectedly: %v", err)
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
	writeTestFile(t, root, "docs/api/feature/DI-101/plan.yaml", `plan:
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
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: []string{"plans"},
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
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: []string{"plans"},
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
		ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: []string{"plans"},
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

	workspace := models.WorkspaceConfig{ID: "workspace", Name: "Repo", Path: root, BaselineBranch: "main", Sources: []string{"plans"}}
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
