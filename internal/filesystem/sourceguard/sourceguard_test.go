package sourceguard

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveAllRejectsEscapesAliasesAndOverlap(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	if _, err := ResolveAll(root, []string{"escape"}); err == nil {
		t.Fatal("expected symlink escape rejection")
	}
	if _, err := ResolveAll(root, []string{"docs", "docs/nested"}); err == nil {
		t.Fatal("expected overlap rejection")
	}
	if err := os.Symlink(filepath.Join(root, "docs"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveAll(root, []string{"docs", "alias"}); err == nil {
		t.Fatal("expected alias rejection")
	}
}
