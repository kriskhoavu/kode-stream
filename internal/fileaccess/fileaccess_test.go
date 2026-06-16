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
	planRoot := filepath.Join(root, "plans", "platform", "PM-001")
	if err := os.MkdirAll(planRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(planRoot, "README.md"), []byte("# PM-001"), 0o644); err != nil {
		t.Fatal(err)
	}
	access := New()
	repo := models.RepositoryConfig{Path: root, PlanDirectories: []string{"plans"}}
	plan := models.PlanDetail{PlanSummary: models.PlanSummary{PlanRoot: "plans/platform/PM-001"}}
	content, err := access.Read(repo, plan, "README_md")
	if err != nil {
		t.Fatal(err)
	}
	if content.Content != "# PM-001" {
		t.Fatalf("content = %q", content.Content)
	}
}

func TestTreeSortsDirectoriesFirstWithNaturalNames(t *testing.T) {
	root := t.TempDir()
	planRoot := filepath.Join(root, "plans", "platform", "PM-001")
	for _, rel := range []string{
		"README.md",
		"file-10.md",
		"file-2.md",
		"design/design-10.md",
		"design/design-2.md",
		"scenario/scenario-1.md",
	} {
		path := filepath.Join(planRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(rel), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	access := New()
	repo := models.RepositoryConfig{Path: root, PlanDirectories: []string{"plans"}}
	plan := models.PlanDetail{PlanSummary: models.PlanSummary{PlanRoot: "plans/platform/PM-001"}}
	tree, err := access.Tree(repo, plan)
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
