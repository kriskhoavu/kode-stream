package git

import (
	"testing"

	"plan-manager/internal/models"
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
