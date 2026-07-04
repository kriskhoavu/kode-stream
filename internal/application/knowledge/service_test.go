package knowledge

import (
	"os"
	"path/filepath"
	"testing"

	"plan-manager/internal/gitadapter"
	knowledgeindex "plan-manager/internal/knowledge"
	"plan-manager/internal/registry"
)

func TestQueriesReturnIndexedPagesAndGuardedMarkdown(t *testing.T) {
	service, workspaceID := newKnowledgeService(t)

	wikis, err := service.Wikis(workspaceID)
	if err != nil || len(wikis) != 1 {
		t.Fatalf("wikis=%#v err=%v", wikis, err)
	}
	pages, warnings, err := service.Pages(workspaceID, "docs")
	if err != nil || len(pages) != 2 || warnings == nil {
		t.Fatalf("pages=%#v warnings=%#v err=%v", pages, warnings, err)
	}
	detail, err := service.Page(workspaceID, "docs", "guide")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Content.Kind != "markdown" || detail.Content.Content == "" || detail.Content.Editable {
		t.Fatalf("detail = %#v", detail)
	}
	graph, err := service.Graph(workspaceID, "docs")
	if err != nil || len(graph.Nodes) != 2 || len(graph.Edges) != 1 {
		t.Fatalf("graph=%#v err=%v", graph, err)
	}
}

func TestQueriesReturnStableMissingAndUnsafeErrors(t *testing.T) {
	service, workspaceID := newKnowledgeService(t)
	if _, err := service.Wikis("missing"); err != ErrWorkspaceNotFound {
		t.Fatalf("workspace err = %v", err)
	}
	if _, _, err := service.Pages(workspaceID, "missing"); err != ErrWikiNotFound {
		t.Fatalf("wiki err = %v", err)
	}
	if _, err := service.Page(workspaceID, "docs", "missing"); err != ErrPageNotFound {
		t.Fatalf("page err = %v", err)
	}
	if _, _, err := service.Pages(workspaceID, "../docs"); err != ErrUnsafePath {
		t.Fatalf("unsafe err = %v", err)
	}
}

func newKnowledgeService(t *testing.T) (*Service, string) {
	t.Helper()
	directory, workspaceRoot := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	indexContent := "---\nslug: index\ntitle: Index\n---\n[[guide]]\n"
	guideContent := "---\nslug: guide\ntitle: Guide\n---\n# Guide\n"
	if err := os.WriteFile(filepath.Join(workspaceRoot, "docs", "index.md"), []byte(indexContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "docs", "guide.md"), []byte(guideContent), 0o644); err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(directory, "workspaces.yaml")
	registryData := "- id: ws\n  name: Workspace\n  path: " + workspaceRoot + "\n  baselineBranch: main\n  sources: [docs]\n"
	if err := os.WriteFile(registryPath, []byte(registryData), 0o600); err != nil {
		t.Fatal(err)
	}
	reg := registry.New(registryPath, gitadapter.New())
	store := knowledgeindex.NewStore(filepath.Join(directory, "knowledge-index.yaml"))
	pages := []knowledgeindex.KnowledgePage{
		{Slug: "index", Title: "Index", Path: "index.md", Domain: "root", Roles: []string{}, Topics: []string{}, Links: []knowledgeindex.KnowledgeLink{{SourceSlug: "index", RawTarget: "guide", Resolution: knowledgeindex.LinkResolved, TargetSlug: "guide"}}, Backlinks: []string{}},
		{Slug: "guide", Title: "Guide", Path: "guide.md", Domain: "root", Roles: []string{}, Topics: []string{}, Links: []knowledgeindex.KnowledgeLink{}, Backlinks: []string{"index"}},
	}
	if err := store.ReplaceWorkspace("ws", []knowledgeindex.KnowledgeWiki{{Root: "docs", DisplayName: "Docs", Pages: pages, Warnings: []knowledgeindex.KnowledgeWarning{}}}); err != nil {
		t.Fatal(err)
	}
	return New(reg, store), "ws"
}
