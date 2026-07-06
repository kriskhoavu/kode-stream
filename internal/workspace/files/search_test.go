package workspacefiles

// Package workspacefiles provides Workspace-owned file operations.

// Package workspacefiles provides bounded workspace file operations.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"plan-manager/internal/common/models"
)

type searchIgnoreChecker struct{}

func (searchIgnoreChecker) Ignored(_ string, paths []string) (map[string]bool, error) {
	ignored := map[string]bool{}
	for _, path := range paths {
		if path == "ignored" || strings.HasPrefix(path, "ignored/") {
			ignored[path] = true
		}
	}
	return ignored, nil
}

func TestSearchFindsUnloadedPathsWithDeterministicOrdering(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "docs", "deep"))
	mustWrite(t, filepath.Join(root, "guide.md"), "root")
	mustWrite(t, filepath.Join(root, "docs", "guide.md"), "docs")
	mustWrite(t, filepath.Join(root, "docs", "deep", "guide-10.md"), "ten")
	mustWrite(t, filepath.Join(root, "docs", "deep", "guide-2.md"), "two")

	result, err := NewWithIgnoreChecker(nil).Search(models.WorkspaceConfig{ID: "ws", Name: "Workspace", Path: root}, "guide.md", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 2 || result.Results[0].Path != "guide.md" || result.Results[1].Path != "docs/guide.md" {
		t.Fatalf("results = %#v", result.Results)
	}
}

func TestSearchRespectsIgnoredAndOutsideSymlinkRules(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "ignored"))
	mustWrite(t, filepath.Join(root, "ignored", "secret.md"), "ignored")
	mustWrite(t, filepath.Join(root, "visible-secret.md"), "visible")
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "secret.md"), "outside")
	if err := os.Symlink(outside, filepath.Join(root, "outside-link")); err != nil {
		t.Fatal(err)
	}
	a := NewWithIgnoreChecker(searchIgnoreChecker{})
	workspace := models.WorkspaceConfig{ID: "ws", Path: root}
	hidden, err := a.Search(workspace, "secret", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(hidden.Results) != 1 || hidden.Results[0].Path != "visible-secret.md" {
		t.Fatalf("hidden results = %#v", hidden.Results)
	}
	shown, err := a.Search(workspace, "secret", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(shown.Results) != 2 || !shown.Results[1].Ignored {
		t.Fatalf("shown results = %#v", shown.Results)
	}
}

func TestSearchReturnsEmptyForBlankQuery(t *testing.T) {
	result, err := NewWithIgnoreChecker(nil).Search(models.WorkspaceConfig{Path: t.TempDir()}, "  ", false)
	if err != nil || len(result.Results) != 0 {
		t.Fatalf("Search blank = %#v, %v", result, err)
	}
}

func TestSearchReportsResultAndTraversalTruncation(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"match-1.md", "match-2.md", "match-3.md", "match-4.md", "other.md"} {
		mustWrite(t, filepath.Join(root, name), name)
	}
	workspace := models.WorkspaceConfig{ID: "ws", Path: root}

	resultLimited := NewWithIgnoreChecker(nil)
	resultLimited.searchResultLimit = 2
	result, err := resultLimited.Search(workspace, "match", false)
	if err != nil || !result.Truncated || len(result.Results) != 2 {
		t.Fatalf("result-limited search = %#v, %v", result, err)
	}

	entryLimited := NewWithIgnoreChecker(nil)
	entryLimited.searchEntryLimit = 2
	result, err = entryLimited.Search(workspace, "missing", false)
	if err != nil || !result.Truncated {
		t.Fatalf("entry-limited search = %#v, %v", result, err)
	}
}
