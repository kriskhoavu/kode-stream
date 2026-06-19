package pathguard

import (
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
