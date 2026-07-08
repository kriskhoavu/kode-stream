package workspacefiles

// Package workspacefiles provides Workspace-owned file operations.

// Package workspacefiles provides bounded workspace file operations.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"kode-stream/internal/common/models"
)

func TestCreateFileAndDirectory(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "docs"))
	workspace := models.WorkspaceConfig{ID: "ws", Path: root}
	a := NewWithIgnoreChecker(nil)

	directory, err := a.CreateDirectory(workspace, models.WorkspaceDirectoryCreateInput{ParentPath: "docs", Name: "guides"})
	if err != nil {
		t.Fatal(err)
	}
	file, err := a.CreateFile(workspace, models.WorkspaceFileCreateInput{ParentPath: "docs/guides", Name: "start.txt", Content: "Start\n"})
	if err != nil {
		t.Fatal(err)
	}
	if directory.Path != "docs/guides" || file.Path != "docs/guides/start.txt" {
		t.Fatalf("unexpected results: %#v %#v", directory, file)
	}
	data, err := os.ReadFile(filepath.Join(root, "docs", "guides", "start.txt"))
	if err != nil || string(data) != "Start\n" {
		t.Fatalf("created file = %q, %v", data, err)
	}
}

func TestCreateRejectsInvalidProtectedUnsupportedAndOccupiedNames(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "docs"))
	mustWrite(t, filepath.Join(root, "docs", "exists.md"), "existing")
	workspace := models.WorkspaceConfig{Path: root}
	a := NewWithIgnoreChecker(nil)

	tests := []struct {
		name  string
		input models.WorkspaceFileCreateInput
		want  error
	}{
		{"traversal name", models.WorkspaceFileCreateInput{ParentPath: "docs", Name: "../escape.md"}, ErrInvalidName},
		{"git name", models.WorkspaceFileCreateInput{ParentPath: "docs", Name: ".git"}, ErrInvalidName},
		{"occupied", models.WorkspaceFileCreateInput{ParentPath: "docs", Name: "exists.md"}, ErrDestinationExists},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := a.CreateFile(workspace, test.input); !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestRenameFileAndDirectoryWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "docs", "old-dir"))
	mustWrite(t, filepath.Join(root, "docs", "old.md"), "old")
	mustWrite(t, filepath.Join(root, "docs", "occupied.md"), "occupied")
	workspace := models.WorkspaceConfig{ID: "ws", Path: root}
	a := NewWithIgnoreChecker(nil)

	file, err := a.Rename(workspace, models.WorkspacePathRenameInput{Path: "docs/old.md", DestinationPath: "docs/new.md"})
	if err != nil {
		t.Fatal(err)
	}
	directory, err := a.Rename(workspace, models.WorkspacePathRenameInput{Path: "docs/old-dir", DestinationPath: "renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if file.Path != "docs/new.md" || directory.Path != "renamed" || len(directory.InvalidatedPaths) != 2 {
		t.Fatalf("unexpected results: %#v %#v", file, directory)
	}
	if _, err := a.Rename(workspace, models.WorkspacePathRenameInput{Path: "docs/new.md", DestinationPath: "docs/occupied.md"}); !errors.Is(err, ErrDestinationExists) {
		t.Fatalf("occupied rename error = %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "docs", "occupied.md"))
	if string(data) != "occupied" {
		t.Fatalf("occupied destination was overwritten: %q", data)
	}
}

func TestRenameRejectsRootProtectedTraversalAndSymlinks(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))
	mustWrite(t, filepath.Join(root, "file.md"), "file")
	outside := filepath.Join(t.TempDir(), "outside.md")
	mustWrite(t, outside, "outside")
	if err := os.Symlink(outside, filepath.Join(root, "link.md")); err != nil {
		t.Fatal(err)
	}
	workspace := models.WorkspaceConfig{Path: root}
	a := NewWithIgnoreChecker(nil)

	for _, input := range []models.WorkspacePathRenameInput{
		{Path: "", DestinationPath: "renamed"},
		{Path: ".git", DestinationPath: "git-copy"},
		{Path: "../outside.md", DestinationPath: "inside.md"},
		{Path: "link.md", DestinationPath: "renamed.md"},
		{Path: "file.md", DestinationPath: "../outside.md"},
	} {
		if _, err := a.Rename(workspace, input); err == nil {
			t.Fatalf("Rename(%#v) succeeded", input)
		}
	}
}
