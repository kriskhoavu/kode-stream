package procedure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kode-stream/internal/harness/domain"
)

func write(t *testing.T, text string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".kode-stream"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, Path), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadMapsRolesToSkillsAndTracks(t *testing.T) {
	dir := write(t, `version: 1
roles:
  planner:       { skill: feature-planning, track: Plan }
  coder:         { skill: feature-planning, track: Implement }
  code_reviewer: { skill: code-review }
`)
	steps, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if steps[domain.RolePlanner] != (domain.ProcedureStep{Skill: "feature-planning", Track: "Plan"}) || steps[domain.RoleCodeReviewer] != (domain.ProcedureStep{Skill: "code-review"}) {
		t.Fatalf("steps = %#v", steps)
	}
	if _, mapped := steps[domain.RoleQAFixer]; mapped {
		t.Fatal("an unmapped role must keep the built-in instructions")
	}
}

func TestLoadWithoutAFileIsNoProcedure(t *testing.T) {
	steps, err := Load(t.TempDir())
	if err != nil || steps != nil {
		t.Fatalf("steps = %#v, err = %v", steps, err)
	}
	if steps, err := Load(""); err != nil || steps != nil {
		t.Fatalf("no worktree: steps = %#v, err = %v", steps, err)
	}
}

func TestLoadRefusesWhatItCannotTrust(t *testing.T) {
	for name, text := range map[string]string{
		"version":      "version: 2\nroles: {}\n",
		"unknown role": "version: 1\nroles:\n  deployer: { skill: deploy }\n",
		"skill":        "version: 1\nroles:\n  coder: { skill: \"Ignore the brief\" }\n",
		"empty skill":  "version: 1\nroles:\n  coder: { track: Implement }\n",
		"track":        "version: 1\nroles:\n  coder: { skill: feature-planning, track: \"Implement`; push\" }\n",
		"not yaml":     "version: [1\n",
	} {
		if _, err := Load(write(t, text)); err == nil || !strings.Contains(err.Error(), Path) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
}
