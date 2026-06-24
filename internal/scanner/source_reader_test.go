package scanner

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"plan-manager/internal/gitadapter"
)

func TestFilesystemSourceReaderReadsRelativeWorkspacePaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "plans/platform/PM-013/README.md", "# PM-013\n")

	reader := NewFilesystemSourceReader(root)
	data, err := reader.ReadFile("plans/platform/PM-013/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# PM-013\n" {
		t.Fatalf("data = %q", data)
	}
	entries, err := reader.ReadDir("plans/platform/PM-013")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "README.md" {
		t.Fatalf("entries = %#v", entries)
	}
	info, err := reader.Stat("plans/platform/PM-013/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() || info.Size() == 0 {
		t.Fatalf("info = %#v", info)
	}
}

func TestGitTreeSourceReaderReadsSnapshotWithoutCheckout(t *testing.T) {
	root := newReaderGitRepo(t)
	writeReaderGitFile(t, root, "plans/main/README.md", "# Main\n")
	readerGitCommit(t, root, "main item")
	readerGitRun(t, root, "switch", "-c", "snapshot")
	writeReaderGitFile(t, root, "plans/platform/PM-013/README.md", "# Snapshot\n")
	writeReaderGitFile(t, root, "plans/platform/PM-013/design/design-01-backend.md", "# Backend\n")
	readerGitCommit(t, root, "snapshot item")
	readerGitRun(t, root, "switch", "main")

	git := gitadapter.New()
	ref, _, err := git.ResolveBranch(root, "snapshot")
	if err != nil {
		t.Fatal(err)
	}
	reader := NewGitTreeSourceReader(root, ref, git)

	data, err := reader.ReadFile("plans/platform/PM-013/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Snapshot\n" {
		t.Fatalf("data = %q", data)
	}
	entries, err := reader.ReadDir("plans/platform/PM-013")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != "README.md" || entries[1].Name() != "design" || !entries[1].IsDir() {
		t.Fatalf("entries = %#v", entries)
	}
	info, err := entries[0].Info()
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() || info.Size() == 0 {
		t.Fatalf("info = %#v", info)
	}
	var walked []string
	if err := reader.WalkDir("plans/platform/PM-013", func(path string, d DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		walked = append(walked, path)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(walked, ",") != "plans/platform/PM-013/README.md,plans/platform/PM-013/design/design-01-backend.md" {
		t.Fatalf("walked = %#v", walked)
	}
	if _, err := reader.Stat("plans/platform/PM-013/missing.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected missing stat error, got %v", err)
	}
	current, err := git.CurrentBranch(root)
	if err != nil {
		t.Fatal(err)
	}
	if current != "main" {
		t.Fatalf("snapshot reader changed branch to %q", current)
	}
}

func newReaderGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	readerGitRun(t, root, "config", "user.name", "Plan Manager")
	readerGitRun(t, root, "config", "user.email", "plan-manager@example.test")
	return root
}

func writeReaderGitFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readerGitCommit(t *testing.T, root, message string) {
	t.Helper()
	readerGitRun(t, root, "add", ".")
	readerGitRun(t, root, "commit", "-m", message)
}

func readerGitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}
