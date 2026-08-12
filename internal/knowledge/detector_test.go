package knowledge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kode-stream/internal/common/models"
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

func TestReadStableBoundedRejectsGrowthAndHonorsCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "page.md")
	if err := os.WriteFile(path, []byte("small"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, warning, err := readStableBounded(context.Background(), path, 4); err != nil || warning != "file_too_large" {
		t.Fatalf("warning=%q err=%v", warning, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := readStableBounded(ctx, path, 1024); err != context.Canceled {
		t.Fatalf("cancellation err=%v", err)
	}
}

func TestStorePublicationPreservesExistingMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "knowledge-index.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nwikis: []\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	if err := store.ReplaceWorkspace("ws", []KnowledgeWiki{{Root: "docs"}}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}

func TestStorePublicationRestoresPreviousGenerationAfterPublicationFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "knowledge-index.yaml")
	store := NewStore(path)
	if err := store.ReplaceWorkspace("ws", []KnowledgeWiki{{Root: "old"}}); err != nil {
		t.Fatal(err)
	}
	store.hooks.afterRename = func() error { return errors.New("injected publication failure") }
	if err := store.ReplaceWorkspace("ws", []KnowledgeWiki{{Root: "new"}}); err == nil {
		t.Fatal("expected publication failure")
	}
	store.hooks = storeHooks{}
	wikis, err := store.List("ws")
	if err != nil || len(wikis) != 1 || wikis[0].Root != "old" {
		t.Fatalf("wikis=%#v err=%v", wikis, err)
	}
}

func TestStoreFirstPublicationRollbackRestoresAbsentState(t *testing.T) {
	for _, hook := range []string{"rename", "directory"} {
		t.Run(hook, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "knowledge-index.yaml")
			store := NewStore(path)
			failure := errors.New("injected publication failure")
			if hook == "rename" {
				store.hooks.afterRename = func() error { return failure }
			} else {
				store.hooks.afterDirectorySync = func() error { return failure }
			}
			if err := store.ReplaceWorkspace("ws", []KnowledgeWiki{{Root: "docs"}}); !errors.Is(err, failure) {
				t.Fatalf("err=%v", err)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("index should be absent after rollback, err=%v", err)
			}
			wikis, err := store.List("ws")
			if err != nil || len(wikis) != 0 {
				t.Fatalf("wikis=%#v err=%v", wikis, err)
			}
		})
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
