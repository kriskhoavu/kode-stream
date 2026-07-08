package workspacefiles

// Package workspacefiles provides Workspace-owned file operations.

// Package workspacefiles provides bounded workspace file operations.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kode-stream/internal/common/models"
)

func TestContentSearchExcludesProtectedIgnoredBinaryAndOutsideFiles(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))
	mustMkdir(t, filepath.Join(root, "ignored"))
	mustWrite(t, filepath.Join(root, ".git", "secret.txt"), "needle")
	mustWrite(t, filepath.Join(root, "ignored", "secret.txt"), "needle")
	if err := os.WriteFile(filepath.Join(root, "binary.txt"), []byte("needle\x00binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "invalid.txt"), []byte{0xff, 0xfe, 'n'}, 0o644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	mustWrite(t, filepath.Join(out, "outside.txt"), "needle")
	if err := os.Symlink(filepath.Join(out, "outside.txt"), filepath.Join(root, "outside.txt")); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "visible.txt"), "needle")

	access := NewWithIgnoreChecker(searchIgnoreChecker{})
	hidden, err := access.ContentSearch(context.Background(), models.WorkspaceConfig{ID: "ws", Path: root}, []models.WorkspaceContentSearchRoot{{}}, models.WorkspaceContentSearchRequest{Query: "needle"}, nil)
	if err != nil || len(hidden.Results) != 1 || hidden.Results[0].Path != "visible.txt" || hidden.SkippedFiles < 2 {
		t.Fatalf("hidden = %#v, err = %v", hidden, err)
	}
	shown, err := access.ContentSearch(context.Background(), models.WorkspaceConfig{ID: "ws", Path: root}, []models.WorkspaceContentSearchRoot{{}}, models.WorkspaceContentSearchRequest{Query: "needle", IncludeIgnored: true}, nil)
	if err != nil || len(shown.Results) != 2 || (!shown.Results[0].Ignored && !shown.Results[1].Ignored) {
		t.Fatalf("shown = %#v, err = %v", shown, err)
	}
}

func TestContentSearchSkipsLargeAndUnreadableFiles(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "large.txt"), "needle in a file over the test limit")
	mustWrite(t, filepath.Join(root, "unreadable.txt"), "needle")
	if err := os.Chmod(filepath.Join(root, "unreadable.txt"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "unreadable.txt"), 0o644) })
	budget := DefaultContentSearchBudget()
	budget.MaxFileSize = 8
	response, err := NewWithIgnoreChecker(nil).ContentSearch(context.Background(), models.WorkspaceConfig{ID: "ws", Path: root}, []models.WorkspaceContentSearchRoot{{}}, models.WorkspaceContentSearchRequest{Query: "needle"}, &budget)
	if err != nil || len(response.Results) != 0 || response.SkippedFiles == 0 {
		t.Fatalf("response = %#v, err = %v", response, err)
	}
}

func TestContentSearchStopsAtFileAndByteBudgets(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "needle")
	mustWrite(t, filepath.Join(root, "b.txt"), "needle")
	workspace := models.WorkspaceConfig{ID: "ws", Path: root}
	fileBudget := DefaultContentSearchBudget()
	fileBudget.MaxFiles = 1
	byFiles, err := NewWithIgnoreChecker(nil).ContentSearch(context.Background(), workspace, []models.WorkspaceContentSearchRoot{{}}, models.WorkspaceContentSearchRequest{Query: "needle"}, &fileBudget)
	if err != nil || !byFiles.Truncated || byFiles.FilesVisited != 1 {
		t.Fatalf("file budget = %#v, err = %v", byFiles, err)
	}
	byteBudget := DefaultContentSearchBudget()
	byteBudget.MaxBytes = 5
	byBytes, err := NewWithIgnoreChecker(nil).ContentSearch(context.Background(), workspace, []models.WorkspaceContentSearchRoot{{}}, models.WorkspaceContentSearchRequest{Query: "needle"}, &byteBudget)
	if err != nil || !byBytes.Truncated || byBytes.BytesRead != 0 {
		t.Fatalf("byte budget = %#v, err = %v", byBytes, err)
	}
}

func TestReadStableContentFileDetectsChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "changing.txt")
	mustWrite(t, path, "before")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	mustWrite(t, path, "after change")
	_, changed, err := readStableContentFile(path, before)
	if err != nil || !changed {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
}

func TestContentSearchCanceledContextDoesNoWork(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "needle")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response, err := NewWithIgnoreChecker(nil).ContentSearch(ctx, models.WorkspaceConfig{ID: "ws", Path: root}, []models.WorkspaceContentSearchRoot{{}}, models.WorkspaceContentSearchRequest{Query: "needle"}, nil)
	if !errors.Is(err, context.Canceled) || response.FilesVisited != 0 {
		t.Fatalf("response = %#v, err = %v", response, err)
	}
}
