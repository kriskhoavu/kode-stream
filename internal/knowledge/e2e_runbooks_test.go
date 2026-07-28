package knowledge

import (
	"os"
	"path/filepath"
	"testing"

	"kode-stream/internal/common/models"
)

func TestMatchesE2ESource(t *testing.T) {
	if !matchesE2ESource([]string{"plans/platform/PM-036/automation/README.md"}, []string{"plans/platform/PM-036/automation/README.md"}) {
		t.Fatal("expected exact source reference to match")
	}
	if matchesE2ESource([]string{"plans/platform/PM-029/automation/README.md"}, []string{"plans/platform/PM-036/automation/README.md"}) {
		t.Fatal("unexpected source reference match")
	}
	if !matchesE2ESource([]string{"plans/platform/PM-036/automation/README.md | Quality coverage"}, []string{"plans/platform/PM-036/automation/README.md"}) {
		t.Fatal("expected annotated source reference to match")
	}
}

func TestKnowledgeE2ERunbookUsesLatestPlanResultSource(t *testing.T) {
	root := t.TempDir()
	older := "plans/platform/PM-020/automation/results/latest.md"
	latest := "plans/platform/PM-036/automation/results/latest.md"
	writeKnowledgeTestFile(t, filepath.Join(root, latest), "Status: blocked\nProvider: playwright\nEvidence: automation/artifacts/blocked.png\n")
	page := KnowledgePage{
		Slug: "e2e-quality-runbook", Title: "Run E2E coverage", Path: "e2e-testing/cross-domain/run-e2e.md",
		Domain: "e2e-testing/cross-domain",
		SourceRefs: []string{
			"plans/platform/PM-020/automation/scenario-01.md",
			"plans/platform/PM-036/automation/scenario-01-run-e2e-coverage.md | Current coverage",
		},
	}
	runbook := (&KnowledgeService{}).e2ERunbook(models.WorkspaceConfig{Path: root}, "wiki", page)
	if runbook.ResultPath != latest {
		t.Fatalf("result path = %q, want %q (older candidate %q)", runbook.ResultPath, latest, older)
	}
	if runbook.LatestResult == nil || runbook.LatestResult.Status != "blocked" || len(runbook.LatestResult.Evidence) != 1 {
		t.Fatalf("latest result = %#v", runbook.LatestResult)
	}
}

func TestKnowledgeE2ERunbookProvidesSafeFallbackBeforeFirstRun(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "wiki", "e2e-testing", "offer"), 0o755); err != nil {
		t.Fatal(err)
	}
	page := KnowledgePage{
		Slug: "e2e-offer-review", Title: "Review offer", Path: "e2e-testing/offer/review.md",
		Domain: "e2e-testing/offer",
	}
	runbook := (&KnowledgeService{}).e2ERunbook(models.WorkspaceConfig{Path: root}, "wiki", page)
	want := "wiki/e2e-testing/offer/automation/results/latest.md"
	if runbook.ResultPath != want || runbook.Diagnostic != "Not run" {
		t.Fatalf("runbook = %#v", runbook)
	}
}

func TestParseE2EResultRejectsUnknownStatus(t *testing.T) {
	if result := parseE2EResult("Status: maybe\n"); result.Status != "not run" {
		t.Fatalf("status = %q", result.Status)
	}
}

func writeKnowledgeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
