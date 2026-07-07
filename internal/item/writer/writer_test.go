package itemwriter

// Package itemwriter persists and refreshes Item domain files.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"plan-manager/internal/common/models"
	"plan-manager/internal/filesystem/content"
	gitadapter "plan-manager/internal/git"
	"plan-manager/internal/item/index"
	"plan-manager/internal/workspace/scanner"
)

func TestSaveMetadataCreatesPlanYAML(t *testing.T) {
	root := t.TempDir()
	itemRoot := filepath.Join(root, "items", "platform", "PM-002")
	if err := os.MkdirAll(itemRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, itemRoot, "README.md", "# PM-002\n")

	writer := New(fileaccess.New(), nil, nil, nil)
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"items"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{
		ItemPath:   "items/platform/PM-002",
		Scope:      "platform",
		Identifier: "PM-002",
		Title:      "Item Editing",
		Status:     models.StatusDraft,
	}}

	if _, err := writer.SaveMetadata(workspace, item, models.ItemMetadataUpdateInput{Status: models.StatusInProgress, Owner: "Khoa Vu", Tags: []string{"items", "items", "edit"}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(itemRoot, "plan.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"plan:", "title: Item Editing", "status: in_progress", "owner: Khoa Vu", "- items", "- edit"} {
		if !strings.Contains(text, want) {
			t.Fatalf("plan.yaml missing %q:\n%s", want, text)
		}
	}
	for _, redundant := range []string{"identifier:", "scope:", "documents:"} {
		if strings.Contains(text, redundant) {
			t.Fatalf("plan.yaml contains redundant %q:\n%s", redundant, text)
		}
	}
}

func TestSaveMetadataRejectsDocsRoot(t *testing.T) {
	writer := New(fileaccess.New(), nil, nil, nil)
	workspace := models.WorkspaceConfig{Path: t.TempDir(), Sources: []string{"docs"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ItemPath: "docs", MetadataSource: "docs"}}
	if _, err := writer.SaveMetadata(workspace, item, models.ItemMetadataUpdateInput{Status: models.StatusDone}); err == nil {
		t.Fatal("expected docs root metadata edit to be rejected")
	}
}

func TestSaveMetadataCompactsLegacyPlanYAML(t *testing.T) {
	root := t.TempDir()
	itemRoot := filepath.Join(root, "plans", "api", "DI-170")
	writeFile(t, itemRoot, "README.md", "# DI-170: Custom Assortment Level 2\n")
	writeFile(t, itemRoot, "design/design-01-backend.md", "# Backend Design\n")
	writeFile(t, itemRoot, "plan.yaml", `schemaVersion: 1
plan:
  ticket: DI-170
  title: Custom Assortment Level 2
  service: api
  status: draft
  owner: null
  tags: [backend]
  targetDate: null
documents:
  - id: design-backend
    role: design
    track: backend
    path: design/design-01-backend.md
    label: Backend Design
    order: 10
`)

	writer := New(fileaccess.New(), nil, nil, nil)
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"plans"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{
		ItemPath: "plans/api/DI-170", Scope: "api", Identifier: "DI-170", Title: "Custom Assortment Level 2", Status: models.StatusDraft,
	}}
	if _, err := writer.SaveMetadata(workspace, item, models.ItemMetadataUpdateInput{Status: models.StatusDone}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(itemRoot, "plan.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if text != "plan:\n    status: done\n    tags:\n        - backend\n" {
		t.Fatalf("unexpected compact plan.yaml:\n%s", text)
	}
}

func TestCreateItemRejectsDuplicate(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "items", "platform", "PM-002")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}

	writer := New(fileaccess.New(), nil, nil, nil)
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"items"}}
	_, err := writer.CreateItem(workspace, models.NewItemInput{
		Source:     "items",
		Scope:      "platform",
		Identifier: "PM-002",
		Title:      "Item Editing",
	})
	if err == nil {
		t.Fatal("expected duplicate item to be rejected")
	}
}

func TestCreateItemWritesOnlyEmptyReadme(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "items"), 0o755); err != nil {
		t.Fatal(err)
	}

	writer := New(fileaccess.New(), nil, nil, nil)
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"items"}}
	if _, err := writer.CreateItem(workspace, models.NewItemInput{
		Source:     "items",
		Scope:      "platform",
		Identifier: "free form item",
		Status:     models.StatusDraft,
		Tags:       []string{"platform"},
	}); err != nil {
		t.Fatal(err)
	}

	itemRoot := filepath.Join(root, "items", "platform", "free form item")
	entries, err := os.ReadDir(itemRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "README.md" {
		t.Fatalf("expected only README.md, got %#v", entries)
	}
	data, err := os.ReadFile(filepath.Join(itemRoot, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("expected an empty README.md, got %q", data)
	}
}

func TestCreateJiraBackedItemWritesReadmeAndMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "items"), 0o755); err != nil {
		t.Fatal(err)
	}

	writer := New(fileaccess.New(), nil, nil, nil)
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"items"}}
	if _, err := writer.CreateItem(workspace, models.NewItemInput{
		Source:        "items",
		Scope:         "platform",
		Identifier:    "PM-025",
		Title:         "Jira First Workspace",
		Status:        models.StatusInProgress,
		Owner:         "Kim",
		Tags:          []string{"jira", "jira", "planning"},
		JiraKey:       "PM-025",
		InitialReadme: "# PM-025: Jira First Workspace\n\n## Jira Context\n\nTicket summary.\n",
	}); err != nil {
		t.Fatal(err)
	}

	itemRoot := filepath.Join(root, "items", "platform", "PM-025")
	readme, err := os.ReadFile(filepath.Join(itemRoot, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(readme); !strings.Contains(got, "## Jira Context") || !strings.Contains(got, "Ticket summary.") {
		t.Fatalf("README content = %q", got)
	}
	data, err := os.ReadFile(filepath.Join(itemRoot, "plan.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"status: in_progress", "owner: Kim", "- jira", "- planning"} {
		if !strings.Contains(text, want) {
			t.Fatalf("plan.yaml missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "identifier:") || strings.Contains(text, "scope:") || strings.Contains(text, "title:") {
		t.Fatalf("plan.yaml should remain compact:\n%s", text)
	}
}

func TestCreateJiraBackedItemRefreshesIndex(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	if err := os.MkdirAll(filepath.Join(root, "items"), 0o755); err != nil {
		t.Fatal(err)
	}

	git := gitadapter.New()
	idx := itemindex.New(filepath.Join(t.TempDir(), "index.yaml"))
	writer := New(fileaccess.New(), scanner.New(git), idx, nil)
	workspace := models.WorkspaceConfig{ID: "workspace-1", Name: "workspace", Path: root, BaselineBranch: "main", Sources: []string{"items"}}
	result, err := writer.CreateItem(workspace, models.NewItemInput{
		Source:        "items",
		Scope:         "platform",
		Identifier:    "PM-026",
		Title:         "Indexed Jira Item",
		Status:        models.StatusDraft,
		Tags:          []string{"jira"},
		JiraKey:       "PM-026",
		InitialReadme: "# PM-026: Indexed Jira Item\n\nIndexed context.\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Item.ID == "" || result.Item.Identifier != "PM-026" {
		t.Fatalf("result item=%#v", result.Item)
	}
	items, err := idx.Query(itemindex.Query{WorkspaceID: workspace.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Identifier != "PM-026" || items[0].Title != "Indexed Jira Item" {
		t.Fatalf("items=%#v", items)
	}
}

func TestSaveMetadataRefreshesIndex(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	itemRoot := filepath.Join(root, "items", "platform", "PM-002")
	writeFile(t, itemRoot, "README.md", "# PM-002\n\nEdit items.\n")

	git := gitadapter.New()
	idx := itemindex.New(filepath.Join(t.TempDir(), "index.yaml"))
	writer := New(fileaccess.New(), scanner.New(git), idx, nil)
	workspace := models.WorkspaceConfig{ID: "workspace-1", Name: "workspace", Path: root, BaselineBranch: "main", Sources: []string{"items"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{
		WorkspaceID: workspace.ID,
		ItemPath:    "items/platform/PM-002",
		Scope:       "platform",
		Identifier:  "PM-002",
		Title:       "Item Editing",
		Status:      models.StatusDraft,
	}}

	if _, err := writer.SaveMetadata(workspace, item, models.ItemMetadataUpdateInput{Status: models.StatusDone}); err != nil {
		t.Fatal(err)
	}
	items, err := idx.Query(itemindex.Query{WorkspaceID: workspace.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Status != models.StatusDone {
		t.Fatalf("status = %q, want done", items[0].Status)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func initGitRepo(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
}
