package fileaccess

// Package fileaccess provides bounded content access.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/fileid"
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
	content, err := access.Read(workspace, item, fileid.Encode("README.md"))
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
	if _, err := access.WriteMarkdown(workspace, item, models.FileSaveInput{FileID: fileid.Encode("README.md"), Content: "changed", ExpectedHash: "stale"}); err == nil {
		t.Fatal("expected stale hash to be rejected")
	}
}

func TestWriteMarkdownUpdatesTextFile(t *testing.T) {
	root := t.TempDir()
	itemRoot := filepath.Join(root, "items", "platform", "PM-001")
	if err := os.MkdirAll(itemRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(itemRoot, "main.go")
	original := []byte("package main\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	access := New()
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"items"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ItemPath: "items/platform/PM-001"}}
	content, err := access.WriteMarkdown(workspace, item, models.FileSaveInput{FileID: fileid.Encode("main.go"), Content: "package planmanager\n", ExpectedHash: contentHash(original)})
	if err != nil {
		t.Fatal(err)
	}
	if !content.Editable || content.Content != "package planmanager\n" {
		t.Fatalf("saved content = %+v", content)
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
	if _, err := access.WriteMarkdown(workspace, item, models.FileSaveInput{FileID: fileid.Encode("escape.md"), Content: "changed"}); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestItemAccessRejectsStoredSourceSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	insideSource := filepath.Join(root, "items")
	itemRoot := filepath.Join(insideSource, "platform", "PM-001")
	if err := os.MkdirAll(itemRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outsideSource := t.TempDir()
	outsideItem := filepath.Join(outsideSource, "platform", "PM-001")
	if err := os.MkdirAll(outsideItem, 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("outside secret")
	outsideFile := filepath.Join(outsideItem, "README.md")
	if err := os.WriteFile(outsideFile, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(insideSource); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideSource, insideSource); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	access := New()
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"items"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ItemPath: "items/platform/PM-001"}}
	if _, err := access.Read(workspace, item, fileid.Encode("README.md")); err == nil {
		t.Fatal("expected escaped source read to be rejected")
	}
	if _, err := access.WriteMarkdown(workspace, item, models.FileSaveInput{FileID: fileid.Encode("README.md"), Content: "changed", ExpectedHash: ContentHash(original)}); err == nil {
		t.Fatal("expected escaped source write to be rejected")
	}
	after, err := os.ReadFile(outsideFile)
	if err != nil || string(after) != string(original) {
		t.Fatalf("outside file changed: %q err=%v", after, err)
	}
}

func TestCollidingLegacyNamesResolveByExactCanonicalID(t *testing.T) {
	root := t.TempDir()
	itemRoot := filepath.Join(root, "items", "platform", "PM-1")
	if err := os.MkdirAll(filepath.Join(itemRoot, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemRoot, "a", "b.md"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemRoot, "a__b.md"), []byte("flat"), 0o644); err != nil {
		t.Fatal(err)
	}
	access := New()
	workspace := models.WorkspaceConfig{Path: root, Sources: []string{"items"}}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{ItemPath: "items/platform/PM-1"}}
	nested, err := access.Read(workspace, item, fileid.Encode("a/b.md"))
	if err != nil {
		t.Fatal(err)
	}
	flat, err := access.Read(workspace, item, fileid.Encode("a__b.md"))
	if err != nil {
		t.Fatal(err)
	}
	if nested.Content != "nested" || flat.Content != "flat" || nested.ID == flat.ID {
		t.Fatalf("nested=%+v flat=%+v", nested, flat)
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

func TestTreeMarksDepthTruncationWithoutRecursingPastBudget(t *testing.T) {
	root := t.TempDir()
	itemRoot := filepath.Join(root, "items", "platform", "PM-001")
	deep := itemRoot
	for index := 0; index < 70; index++ {
		deep = filepath.Join(deep, fmt.Sprintf("d%02d", index))
	}
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	access := New()
	tree, err := access.Tree(
		models.WorkspaceConfig{Path: root, Sources: []string{"items"}},
		models.ItemDetail{ItemSummary: models.ItemSummary{ItemPath: "items/platform/PM-001"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	current := tree[0]
	for !current.Truncated {
		if len(current.Children) != 1 {
			t.Fatalf("expected one bounded child at %s: %#v", current.Path, current.Children)
		}
		current = current.Children[0]
	}
	if len(current.Children) != 0 {
		t.Fatalf("truncated node contains children: %#v", current)
	}
}

func nodeNames(nodes []models.FileNode) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}
	return names
}
