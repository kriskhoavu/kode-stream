package workspacefiles

// Package workspacefiles provides Workspace-owned file operations.

// Package workspacefiles provides bounded workspace file operations.

import (
	"os"
	"path/filepath"
	"testing"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/content"
)

type fakeIgnoreChecker struct {
	paths []string
}

func (f *fakeIgnoreChecker) Ignored(_ string, paths []string) (map[string]bool, error) {
	f.paths = append([]string(nil), paths...)
	return map[string]bool{"ignored.txt": true}, nil
}

func TestListReturnsImmediateNaturallySortedEntries(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "dir10", "deep"))
	mustMkdir(t, filepath.Join(root, "dir2"))
	mustWrite(t, filepath.Join(root, ".hidden.md"), "hidden")
	mustWrite(t, filepath.Join(root, "file10.txt"), "ten")
	mustWrite(t, filepath.Join(root, "file2.md"), "two")
	mustWrite(t, filepath.Join(root, "ignored.txt"), "ignored")
	ignore := &fakeIgnoreChecker{}

	listing, err := NewWithIgnoreChecker(ignore).List(models.WorkspaceConfig{ID: "ws", Path: root}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(ignore.paths), 6; got != want {
		t.Fatalf("ignore paths = %d, want %d", got, want)
	}
	if got, want := len(listing.Entries), 5; got != want {
		t.Fatalf("entries = %d, want %d", got, want)
	}
	if listing.HiddenCount != 1 {
		t.Fatalf("hiddenCount = %d, want 1", listing.HiddenCount)
	}
	wantNames := []string{"dir2", "dir10", ".hidden.md", "file2.md", "file10.txt"}
	for i, want := range wantNames {
		if listing.Entries[i].Name != want {
			t.Fatalf("entry %d = %q, want %q", i, listing.Entries[i].Name, want)
		}
	}
	if listing.Entries[0].HasChildren || !listing.Entries[1].HasChildren {
		t.Fatal("directory hasChildren values are incorrect")
	}
	if !listing.Entries[2].Hidden || !listing.Entries[3].Editable || listing.Entries[3].Language != "markdown" {
		t.Fatal("file metadata is incorrect")
	}
}

func TestListIncludesIgnoredEntriesOnRequest(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "ignored.txt"), "ignored")
	listing, err := NewWithIgnoreChecker(&fakeIgnoreChecker{}).List(models.WorkspaceConfig{ID: "ws", Path: root}, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Entries) != 1 || !listing.Entries[0].Ignored {
		t.Fatalf("ignored entry not returned: %#v", listing.Entries)
	}
}

func TestWriteMarkdownUpdatesTextFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.go")
	original := []byte("package main\n")
	mustWrite(t, path, string(original))

	content, err := NewWithIgnoreChecker(nil).WriteMarkdown(models.WorkspaceConfig{Path: root}, models.WorkspaceFileSaveInput{
		Path:         "main.go",
		Content:      "package planmanager\n",
		ExpectedHash: fileaccess.ContentHash(original),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !content.Editable || content.Content != "package planmanager\n" {
		t.Fatalf("saved content = %+v", content)
	}
}

func TestResolveRejectsProtectedTraversalAndOutsideSymlink(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))
	mustWrite(t, filepath.Join(root, "ok.md"), "ok")
	outside := filepath.Join(t.TempDir(), "outside.md")
	mustWrite(t, outside, "outside")
	if err := os.Symlink(outside, filepath.Join(root, "escape.md")); err != nil {
		t.Fatal(err)
	}
	a := NewWithIgnoreChecker(nil)
	workspace := models.WorkspaceConfig{Path: root}
	for _, path := range []string{"../outside.md", ".git/config", "escape.md"} {
		if _, _, err := a.ResolveFile(workspace, path); err == nil {
			t.Fatalf("ResolveFile(%q) succeeded", path)
		}
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
