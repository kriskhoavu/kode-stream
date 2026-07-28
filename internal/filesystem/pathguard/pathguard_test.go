package pathguard

// Package pathguard validates workspace-relative paths.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeJoinRejectsTraversal(t *testing.T) {
	if _, err := SafeJoin(t.TempDir(), "../secret.md"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestSafeJoinReturnsAbsolutePathInsideRoot(t *testing.T) {
	root := t.TempDir()
	got, err := SafeJoin(root, "plans/PM-003/README.md")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "plans", "PM-003", "README.md")
	if got != want {
		t.Fatalf("SafeJoin() = %q, want %q", got, want)
	}
}

func TestValidateMarkdownFileRejectsUnsupportedAndSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "runbook.txt"), []byte("not markdown"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateMarkdownFile(root, "runbook.txt"); err == nil {
		t.Fatal("expected unsupported extension to be rejected")
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("# Outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "runbook.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := ValidateMarkdownFile(root, "runbook.md"); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestValidateMarkdownTargetAllowsMissingFileAndRejectsSymlinkParent(t *testing.T) {
	root := t.TempDir()
	if _, err := ValidateMarkdownTarget(root, "automation/results/latest.md"); err != nil {
		t.Fatalf("missing target should be valid: %v", err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "redirect")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := ValidateMarkdownTarget(root, "redirect/latest.md"); err == nil {
		t.Fatal("expected symlink parent escape to be rejected")
	}
}

func TestValidateSourcePathsRejectsInvalidPaths(t *testing.T) {
	for _, paths := range [][]string{
		{},
		{"../secret.md"},
		{"/tmp/secret.md"},
		{"src/main.go"},
	} {
		if err := ValidateSourcePaths([]string{"plans"}, paths); err == nil {
			t.Fatalf("expected %#v to be rejected", paths)
		}
	}
}

func TestValidateSourcePathsAllowsRegisteredSourcePaths(t *testing.T) {
	if err := ValidateSourcePaths([]string{"plans", "docs"}, []string{"plans/platform/PM-003/README.md", "docs/guide.md"}); err != nil {
		t.Fatalf("expected paths to be valid: %v", err)
	}
}
