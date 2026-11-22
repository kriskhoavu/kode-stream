package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceWritesContentAndMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.yaml")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Replace(path, ".file-*", []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(data) != "new" || info.Mode().Perm() != 0o600 {
		t.Fatalf("got %q %v", data, info.Mode().Perm())
	}
	assertOnlyFile(t, dir)
}

func TestReplaceKeepsOriginalAndRemovesTemporaryWhenRenameFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.yaml")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("rename failed")
	err := ReplaceWith(path, ".file-*", []byte("new"), 0o644, func(string, string) error { return failure })
	if !errors.Is(err, failure) {
		t.Fatalf("err = %v", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "old" {
		t.Fatalf("content = %q", data)
	}
	assertOnlyFile(t, dir)
}

func TestWriteTempReturnsPathWhenAHookFails(t *testing.T) {
	dir := t.TempDir()
	failure := errors.New("after sync")
	path, err := WriteTemp(dir, ".file-*", 0o600, []byte("data"), Hooks{AfterSync: func() error { return failure }})
	if !errors.Is(err, failure) || path == "" {
		t.Fatalf("path %q err %v", path, err)
	}
}

func assertOnlyFile(t *testing.T, dir string) {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries", len(entries))
	}
}
