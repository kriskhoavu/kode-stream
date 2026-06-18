package fileaccess

import (
	"os"
	"path/filepath"
	"testing"

	"plan-manager/internal/models"
)

func TestSafeJoinRejectsTraversal(t *testing.T) {
	if _, err := safeJoin(t.TempDir(), "../secret.md"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestReadStaysInsidePlanDirectory(t *testing.T) {
	root := t.TempDir()
	itemRoot := filepath.Join(root, "items", "platform", "PM-001")
	if err := os.MkdirAll(itemRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemRoot, "README.md"), []byte("# PM-001"), 0o644); err != nil {
		t.Fatal(err)
	}
	access := New()
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"items"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ItemPath: "items/platform/PM-001"}}
	content, err := access.Read(workspace, item, "README_md")
	if err != nil {
		t.Fatal(err)
	}
	if content.Content != "# PM-001" {
		t.Fatalf("content = %q", content.Content)
	}
	if content.Hash == "" {
		t.Fatal("expected content hash")
	}
}

func TestWriteMarkdownRejectsStaleHash(t *testing.T) {
	root := t.TempDir()
	itemRoot := filepath.Join(root, "items", "platform", "PM-001")
	if err := os.MkdirAll(itemRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemRoot, "README.md"), []byte("# PM-001"), 0o644); err != nil {
		t.Fatal(err)
	}

	access := New()
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"items"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ItemPath: "items/platform/PM-001"}}
	if _, err := access.WriteMarkdown(workspace, item, models.FileSaveInput{FileID: "README_md", Content: "changed", ExpectedHash: "stale"}); err == nil {
		t.Fatal("expected stale hash to be rejected")
	}
}

func TestWriteMarkdownRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	itemRoot := filepath.Join(root, "items", "platform", "PM-001")
	if err := os.MkdirAll(itemRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "secret.md")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(itemRoot, "escape.md")); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	access := New()
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"items"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ItemPath: "items/platform/PM-001"}}
	if _, err := access.WriteMarkdown(workspace, item, models.FileSaveInput{FileID: "escape_md", Content: "changed"}); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestTreeSortsDirectoriesFirstWithNaturalNames(t *testing.T) {
	root := t.TempDir()
	itemRoot := filepath.Join(root, "items", "platform", "PM-001")
	for _, rel := range []string{
		"README.md",
		"file-10.md",
		"file-2.md",
		"design/design-10.md",
		"design/design-2.md",
		"scenario/scenario-1.md",
	} {
		path := filepath.Join(itemRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(rel), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	access := New()
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"items"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ItemPath: "items/platform/PM-001"}}
	tree, err := access.Tree(workspace, item)
	if err != nil {
		t.Fatal(err)
	}

	got := nodeNames(tree)
	want := []string{"design", "scenario", "file-2.md", "file-10.md", "README.md"}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("root node %d = %q, want %q; all nodes: %#v", i, got[i], name, got)
		}
	}
	design := tree[0]
	gotDesign := nodeNames(design.Children)
	if gotDesign[0] != "design-2.md" || gotDesign[1] != "design-10.md" {
		t.Fatalf("design children = %#v", gotDesign)
	}
}

func nodeNames(nodes []models.FileNode) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}
	return names
}
