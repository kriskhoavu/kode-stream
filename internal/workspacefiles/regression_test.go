package workspacefiles

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"plan-manager/internal/models"
)

func TestListUsesGitIgnoreAndNeverExposesGitInternals(t *testing.T) {
	root := initGitWorkspace(t)
	mustWrite(t, filepath.Join(root, ".gitignore"), "ignored/\n*.tmp\n")
	mustMkdir(t, filepath.Join(root, "ignored"))
	mustWrite(t, filepath.Join(root, "ignored", "nested.txt"), "ignored")
	mustWrite(t, filepath.Join(root, "debug.tmp"), "ignored")
	mustWrite(t, filepath.Join(root, "visible.txt"), "visible")
	workspace := models.WorkspaceConfig{ID: "ws", Path: root}

	listing, err := New().List(workspace, "", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range listing.Entries {
		if entry.Name == ".git" || entry.Name == "ignored" || entry.Name == "debug.tmp" {
			t.Fatalf("protected or ignored entry was exposed: %#v", entry)
		}
	}
	if listing.HiddenCount != 3 {
		t.Fatalf("hiddenCount = %d, want 3", listing.HiddenCount)
	}

	withIgnored, err := New().List(workspace, "", true)
	if err != nil {
		t.Fatal(err)
	}
	foundIgnored := 0
	for _, entry := range withIgnored.Entries {
		if entry.Name == ".git" {
			t.Fatal(".git was exposed with includeIgnored")
		}
		if entry.Ignored {
			foundIgnored++
		}
	}
	if foundIgnored != 2 {
		t.Fatalf("ignored entries = %d, want 2", foundIgnored)
	}
}

func TestLargeDirectoryListingDoesNotLoadDeepDescendants(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 500; i++ {
		mustWrite(t, filepath.Join(root, fmt.Sprintf("file-%03d.txt", i)), "value")
	}
	deep := filepath.Join(root, "deep")
	for i := 0; i < 50; i++ {
		deep = filepath.Join(deep, "child")
	}
	mustMkdir(t, deep)
	mustWrite(t, filepath.Join(deep, "leaf.txt"), "leaf")

	listing, err := NewWithIgnoreChecker(nil).List(models.WorkspaceConfig{ID: "ws", Path: root}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Entries) != 501 {
		t.Fatalf("entries = %d, want 501 immediate children", len(listing.Entries))
	}
	if listing.Entries[0].Name != "deep" || !listing.Entries[0].HasChildren {
		t.Fatalf("deep directory metadata = %#v", listing.Entries[0])
	}
}

func TestResolveRejectsWrongTypesAndMissingPaths(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "directory"))
	mustWrite(t, filepath.Join(root, "file.md"), "file")
	a := NewWithIgnoreChecker(nil)
	workspace := models.WorkspaceConfig{Path: root}
	if _, err := a.List(workspace, "file.md", false); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("listing file error = %v", err)
	}
	if _, _, err := a.ResolveFile(workspace, "directory"); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("resolving directory error = %v", err)
	}
	if _, _, err := a.ResolveFile(workspace, "missing.md"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resolving missing error = %v", err)
	}
}

func initGitWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	return root
}
