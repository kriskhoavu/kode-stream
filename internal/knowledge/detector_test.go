package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"plan-manager/internal/common/models"
)

type detectorIgnoreChecker struct{}

func (detectorIgnoreChecker) Ignored(_ string, paths []string) (map[string]bool, error) {
	ignored := map[string]bool{}
	for _, path := range paths {
		if path == "docs/private.md" {
			ignored[path] = true
		}
	}
	return ignored, nil
}

func TestDetectorFindsOnlyRegisteredCompatibleWikiSources(t *testing.T) {
	root := t.TempDir()
	writePage(t, root, "docs/index.md", "index", "Index", "[[guide]]")
	writePage(t, root, "docs/guide.md", "guide", "Guide", "[Index](index.md)")
	writePage(t, root, "other/index.md", "other", "Other", "")
	workspace := models.WorkspaceConfig{ID: "ws", Path: root, Sources: []string{"docs", "missing"}}

	wikis, err := NewDetector().DetectWorkspace(context.Background(), workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(wikis) != 1 || wikis[0].Root != "docs" || len(wikis[0].Pages) != 2 {
		t.Fatalf("wikis = %#v", wikis)
	}
	if len(wikis[0].Pages[1].Backlinks) != 1 {
		t.Fatalf("pages = %#v", wikis[0].Pages)
	}
}

func TestDetectorRejectsSymlinkSourceEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	writePage(t, outside, "index.md", "outside", "Outside", "")
	if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	_, err := NewDetector().DetectWorkspace(context.Background(), models.WorkspaceConfig{ID: "ws", Path: root, Sources: []string{"docs"}})
	if err == nil {
		t.Fatal("expected escaped source rejection")
	}
}

func TestDetectorPreservesPartialPagesForOversizedFiles(t *testing.T) {
	root := t.TempDir()
	writePage(t, root, "docs/index.md", "index", "Index", "")
	writePage(t, root, "docs/large.md", "large", "Large", strings.Repeat("long body ", 30))
	detector := NewDetector()
	detector.Limits.MaxFileBytes = 80
	wikis, err := detector.DetectWorkspace(context.Background(), models.WorkspaceConfig{ID: "ws", Path: root, Sources: []string{"docs"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(wikis) != 1 || len(wikis[0].Warnings) == 0 {
		t.Fatalf("wikis = %#v", wikis)
	}
}

func TestDetectorExcludesGitIgnoredMarkdownAndDeduplicatesSources(t *testing.T) {
	root := t.TempDir()
	writePage(t, root, "docs/index.md", "index", "Index", "")
	writePage(t, root, "docs/private.md", "private", "Private", "")
	detector := NewDetector()
	detector.ignore = detectorIgnoreChecker{}
	wikis, err := detector.DetectWorkspace(context.Background(), models.WorkspaceConfig{ID: "ws", Path: root, Sources: []string{"docs", "docs"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(wikis) != 1 || len(wikis[0].Pages) != 1 || wikis[0].Pages[0].Slug != "index" {
		t.Fatalf("wikis = %#v", wikis)
	}
}

func TestStoreReplacesWorkspaceAtomicallyAndKeepsOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "knowledge-index.yaml")
	store := NewStore(path)
	if err := store.ReplaceWorkspace("one", []KnowledgeWiki{{Root: "docs"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceWorkspace("two", []KnowledgeWiki{{Root: "wiki"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceWorkspace("one", []KnowledgeWiki{{Root: "new"}}); err != nil {
		t.Fatal(err)
	}
	wikis, err := store.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(wikis) != 2 || wikis[0].Root != "new" || wikis[1].Root != "wiki" {
		t.Fatalf("wikis = %#v", wikis)
	}
}

func writePage(t *testing.T, root, relative, slug, title, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nslug: " + slug + "\ntitle: " + title + "\n---\n" + body
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
