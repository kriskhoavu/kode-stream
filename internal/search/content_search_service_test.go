package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"kode-stream/internal/common/models"
	"kode-stream/internal/workspace/files"
)

type fakeRegistry struct{ workspaces []models.WorkspaceConfig }

func (f fakeRegistry) List() ([]models.WorkspaceConfig, error) { return f.workspaces, nil }
func (f fakeRegistry) Get(id string) (models.WorkspaceConfig, bool, error) {
	for _, workspace := range f.workspaces {
		if workspace.ID == id {
			return workspace, true, nil
		}
	}
	return models.WorkspaceConfig{}, false, nil
}

type fakeIndex struct{ item models.ItemDetail }

func (f fakeIndex) Get(id string) (models.ItemDetail, bool, error) {
	return f.item, id == f.item.ID, nil
}

func TestSearchItemUsesOnlyTheItemRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "plans/PM-1/readme.md", "private needle")
	writeFile(t, root, "plans/PM-2/readme.md", "sibling needle")
	workspace := models.WorkspaceConfig{ID: "ws", Name: "Workspace", Path: root, Sources: []string{"plans"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "item-1", WorkspaceID: "ws", ItemPath: "plans/PM-1"}}
	service := NewContentSearchService(fakeRegistry{[]models.WorkspaceConfig{workspace}}, fakeIndex{item}, workspacefiles.NewWithIgnoreChecker(nil))
	response, err := service.SearchItem(context.Background(), "item-1", models.WorkspaceContentSearchRequest{Query: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || response.Results[0].Path != "plans/PM-1/readme.md" || response.Results[0].ItemID != "item-1" || response.Results[0].FileID != "readme_md" {
		t.Fatalf("response = %#v", response)
	}
}

func TestSearchExplorerResolvesSourcesAndAllModes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "root.txt", "root needle")
	writeFile(t, root, "plans/a.md", "source needle")
	writeFile(t, root, "docs/b.md", "nested needle")
	workspace := models.WorkspaceConfig{ID: "ws", Path: root, Sources: []string{"plans", "plans", "docs"}}
	service := NewContentSearchService(fakeRegistry{[]models.WorkspaceConfig{workspace}}, fakeIndex{}, workspacefiles.NewWithIgnoreChecker(nil))
	sources, err := service.SearchExplorer(context.Background(), ModeSources, "", models.WorkspaceContentSearchRequest{Query: "needle"})
	if err != nil || len(sources.Results) != 2 {
		t.Fatalf("sources = %#v, err = %v", sources, err)
	}
	all, err := service.SearchExplorer(context.Background(), ModeAll, "ws", models.WorkspaceContentSearchRequest{Query: "needle"})
	if err != nil || len(all.Results) != 3 {
		t.Fatalf("all = %#v, err = %v", all, err)
	}
}

func TestCanonicalRootsDeduplicatesNestedRootsAndSkipsMissingSources(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plans", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	workspace := models.WorkspaceConfig{Path: root}
	roots, err := canonicalRoots(workspace, []string{"plans/nested", "missing", "plans"}, true)
	if err != nil || len(roots) != 1 || roots[0].Path != "plans" {
		t.Fatalf("roots = %#v, err = %v", roots, err)
	}
}

func TestSearchExplorerSharesBudgetAcrossWorkspaces(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	writeFile(t, first, "a.md", "needle")
	writeFile(t, second, "b.md", "needle")
	registry := fakeRegistry{[]models.WorkspaceConfig{{ID: "one", Path: first}, {ID: "two", Path: second}}}
	service := NewContentSearchService(registry, fakeIndex{}, workspacefiles.NewWithIgnoreChecker(nil))
	response, err := service.SearchExplorer(context.Background(), ModeAll, "", models.WorkspaceContentSearchRequest{Query: "needle"})
	if err != nil || response.FilesVisited != 2 || len(response.Results) != 2 {
		t.Fatalf("response = %#v, err = %v", response, err)
	}
}

func writeFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
