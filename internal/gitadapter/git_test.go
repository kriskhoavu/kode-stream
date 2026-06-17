package gitadapter

import (
	"testing"

	"plan-manager/internal/models"
)

func TestParseBranchLine(t *testing.T) {
	status := models.GitStatus{}
	parseBranchLine(&status, "feature/PM-002...origin/feature/PM-002 [ahead 2, behind 1]")

	if status.Branch != "feature/PM-002" {
		t.Fatalf("branch = %q", status.Branch)
	}
	if status.Upstream != "origin/feature/PM-002" {
		t.Fatalf("upstream = %q", status.Upstream)
	}
	if status.Ahead != 2 || status.Behind != 1 {
		t.Fatalf("ahead/behind = %d/%d", status.Ahead, status.Behind)
	}
}

func TestParseChangeLine(t *testing.T) {
	change := parseChangeLine(" M plans/platform/PM-002/README.md")
	if change.Path != "plans/platform/PM-002/README.md" {
		t.Fatalf("path = %q", change.Path)
	}
	if change.Status != models.GitChangeModified || change.Staged {
		t.Fatalf("change = %#v", change)
	}

	renamed := parseChangeLine("R  old.md -> new.md")
	if renamed.Status != models.GitChangeRenamed || renamed.OldPath != "old.md" || renamed.Path != "new.md" || !renamed.Staged {
		t.Fatalf("renamed = %#v", renamed)
	}

	conflicted := parseChangeLine("UU plans/platform/PM-002/README.md")
	if conflicted.Status != models.GitChangeConflicted || !conflicted.Conflict {
		t.Fatalf("conflicted = %#v", conflicted)
	}
}
