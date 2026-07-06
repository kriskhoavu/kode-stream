package git

// Git service contract tests.

import (
	"testing"

	"plan-manager/internal/common/models"
)

func TestValidatePathsRequiresConfiguredSourcePaths(t *testing.T) {
	workspace := models.WorkspaceConfig{Sources: []string{"plans", "docs"}}
	if err := ValidatePaths(workspace, []string{"plans/platform/PM-003/README.md", "docs/guide.md"}); err != nil {
		t.Fatalf("expected valid paths: %v", err)
	}
}

func TestValidatePathsRejectsEmptyEscapedAndUnregisteredPaths(t *testing.T) {
	workspace := models.WorkspaceConfig{Sources: []string{"plans"}}
	for _, paths := range [][]string{
		{},
		{"../secret.md"},
		{"/tmp/secret.md"},
		{"src/main.go"},
	} {
		if err := ValidatePaths(workspace, paths); err == nil {
			t.Fatalf("expected %#v to be rejected", paths)
		}
	}
}

func TestNormalizeWorkspaceBranchesSortsDeduplicatesAndKeepsCurrent(t *testing.T) {
	result := normalizeWorkspaceBranches("workspace", "feature/current", []string{"main", "alpha", "main", " "})
	want := []string{"alpha", "feature/current", "main"}
	if result.WorkspaceID != "workspace" || result.Current != "feature/current" || len(result.Branches) != len(want) {
		t.Fatalf("branches = %#v", result)
	}
	for index := range want {
		if result.Branches[index] != want[index] {
			t.Fatalf("branch %d = %q, want %q", index, result.Branches[index], want[index])
		}
	}
}
