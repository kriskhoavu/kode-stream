package planwriter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"plan-manager/internal/fileaccess"
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/models"
	"plan-manager/internal/planindex"
	"plan-manager/internal/scanner"
)

func TestSaveMetadataCreatesPlanYAML(t *testing.T) {
	root := t.TempDir()
	planRoot := filepath.Join(root, "plans", "platform", "PM-002")
	if err := os.MkdirAll(planRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, planRoot, "README.md", "# PM-002\n")

	writer := New(fileaccess.New(), nil, nil, nil)
	repo := models.RepositoryConfig{Path: root, PlanDirectories: []string{"plans"}}
	plan := models.PlanDetail{PlanSummary: models.PlanSummary{
		PlanRoot: "plans/platform/PM-002",
		Service:  "platform",
		Ticket:   "PM-002",
		Title:    "Plan Editing",
		Status:   models.StatusDraft,
	}}

	if _, err := writer.SaveMetadata(repo, plan, models.PlanMetadataUpdateInput{Status: models.StatusInProgress, Owner: "Khoa Vu", Tags: []string{"plans", "plans", "edit"}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(planRoot, "plan.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"ticket: PM-002", "status: in_progress", "owner: Khoa Vu", "- plans", "- edit"} {
		if !strings.Contains(text, want) {
			t.Fatalf("plan.yaml missing %q:\n%s", want, text)
		}
	}
}

func TestSaveMetadataRejectsDocsRoot(t *testing.T) {
	writer := New(fileaccess.New(), nil, nil, nil)
	repo := models.RepositoryConfig{Path: t.TempDir(), PlanDirectories: []string{"docs"}}
	plan := models.PlanDetail{PlanSummary: models.PlanSummary{PlanRoot: "docs", MetadataSource: "docs"}}
	if _, err := writer.SaveMetadata(repo, plan, models.PlanMetadataUpdateInput{Status: models.StatusDone}); err == nil {
		t.Fatal("expected docs root metadata edit to be rejected")
	}
}

func TestCreatePlanRejectsDuplicate(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "plans", "platform", "PM-002")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}

	writer := New(fileaccess.New(), nil, nil, nil)
	repo := models.RepositoryConfig{Path: root, PlanDirectories: []string{"plans"}}
	_, err := writer.CreatePlan(repo, models.NewPlanInput{
		PlanDirectory: "plans",
		Service:       "platform",
		Ticket:        "PM-002",
		Title:         "Plan Editing",
	})
	if err == nil {
		t.Fatal("expected duplicate plan to be rejected")
	}
}

func TestCreatePlanWritesStarterFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}

	writer := New(fileaccess.New(), nil, nil, nil)
	repo := models.RepositoryConfig{Path: root, PlanDirectories: []string{"plans"}}
	if _, err := writer.CreatePlan(repo, models.NewPlanInput{
		PlanDirectory: "plans",
		Service:       "platform",
		Ticket:        "PM-003",
		Title:         "Next Plan",
		Status:        models.StatusIdeas,
		Tags:          []string{"platform"},
	}); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"README.md", "scenario/scenario-00-overview.md", "design/design-01-backend.md", "design/design-02-frontend.md", "implementation-plan.md", "plan.yaml"} {
		if _, err := os.Stat(filepath.Join(root, "plans", "platform", "PM-003", filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected %s: %v", rel, err)
		}
	}
}

func TestSaveMetadataRefreshesIndex(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	planRoot := filepath.Join(root, "plans", "platform", "PM-002")
	writeFile(t, planRoot, "README.md", "# PM-002\n\nEdit plans.\n")

	git := gitadapter.New()
	idx := planindex.New(filepath.Join(t.TempDir(), "index.yaml"))
	writer := New(fileaccess.New(), scanner.New(git), idx, nil)
	repo := models.RepositoryConfig{ID: "repo-1", Name: "repo", Path: root, BaselineBranch: "main", PlanDirectories: []string{"plans"}}
	plan := models.PlanDetail{PlanSummary: models.PlanSummary{
		RepositoryID: repo.ID,
		PlanRoot:     "plans/platform/PM-002",
		Service:      "platform",
		Ticket:       "PM-002",
		Title:        "Plan Editing",
		Status:       models.StatusDraft,
	}}

	if _, err := writer.SaveMetadata(repo, plan, models.PlanMetadataUpdateInput{Status: models.StatusDone}); err != nil {
		t.Fatal(err)
	}
	plans, err := idx.Query(planindex.Query{RepositoryID: repo.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 {
		t.Fatalf("plans = %d, want 1", len(plans))
	}
	if plans[0].Status != models.StatusDone {
		t.Fatalf("status = %q, want done", plans[0].Status)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func initGitRepo(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
}
