package item

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/e2eresult"
	gitadapter "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/workspace/registry"
)

func TestParseE2ELatestResult(t *testing.T) {
	result := parseE2ELatestResult("# E2E Result\n\nStatus: failed\nProvider: playwright\nEnvironment: staging\nFailed step: Open offer\nEvidence: automation/artifacts/failure.png\n")
	if result.Status != "failed" || result.Provider != "playwright" || result.Environment != "staging" || result.FailedStep != "Open offer" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(result.Evidence) != 1 || result.Evidence[0] != "automation/artifacts/failure.png" {
		t.Fatalf("unexpected evidence: %#v", result.Evidence)
	}
}

func TestParseE2ELatestResultDefaultsToNotRun(t *testing.T) {
	if result := parseE2ELatestResult("# Placeholder\n"); result.Status != "not run" {
		t.Fatalf("status = %q, want not run", result.Status)
	}
	if result := parseE2ELatestResult("Status: unknown\n"); result.Status != "not run" {
		t.Fatalf("invalid status = %q, want not run", result.Status)
	}
}

func TestE2ERunbooksBoundLatestResultBeforeParsing(t *testing.T) {
	service, workspace, item := e2eRunbookTestService(t)
	automation := filepath.Join(workspace.Path, item.ItemPath, "automation")
	writeTestFile(t, filepath.Join(automation, "scenario-01-bounded.md"), "# Bounded\n")
	writeTestFile(t, filepath.Join(automation, "results", "latest.md"), strings.Repeat("x", e2eresult.MaxBytes+1))
	result, _, err := service.E2ERunbooks(item.ID)
	if err != nil || len(result.Runbooks) != 1 || result.Runbooks[0].LatestResult == nil || result.Runbooks[0].LatestResult.Freshness != "unknown" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestE2ERunbooksBoundRunbookBeforeFingerprinting(t *testing.T) {
	service, workspace, item := e2eRunbookTestService(t)
	automation := filepath.Join(workspace.Path, item.ItemPath, "automation")
	writeTestFile(t, filepath.Join(automation, "scenario-01-large.md"), strings.Repeat("# x\n", e2eresult.MaxRunbookBytes))
	writeTestFile(t, filepath.Join(automation, "results", "latest.md"), `{}`)
	result, _, err := service.E2ERunbooks(item.ID)
	if err != nil || len(result.Runbooks) != 1 || !strings.Contains(result.Runbooks[0].Diagnostic, "exceeds") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestIsE2EScenarioRunbook(t *testing.T) {
	if !isE2EScenarioRunbook("scenario-01-review-offer.md") {
		t.Fatal("expected scenario runbook to be included")
	}
	if isE2EScenarioRunbook("README.md") || isE2EScenarioRunbook("results/latest.md") {
		t.Fatal("expected hub and result files to be excluded")
	}
}

func TestE2ERunbooksDiscoversOrderedScenariosAndHubSource(t *testing.T) {
	service, workspace, item := e2eRunbookTestService(t)
	automation := filepath.Join(workspace.Path, item.ItemPath, "automation")
	writeTestFile(t, filepath.Join(automation, "README.md"), "# UI Automation\n")
	first := "# First journey\n"
	writeTestFile(t, filepath.Join(automation, "scenario-02-second.md"), "# Second journey\n")
	writeTestFile(t, filepath.Join(automation, "scenario-01-first.md"), first)
	writeTestFile(t, filepath.Join(automation, "results", "latest.md"), `{"version":1,"status":"passed","runbookFingerprint":"`+e2eresult.Fingerprint([]byte(first))+`","recordedAt":"2026-08-13T00:00:00Z","evidence":["plans/platform/PM-036/automation/artifacts/pass.png"]}`)

	result, sources, err := service.E2ERunbooks(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Runbooks) != 2 || result.Runbooks[0].Title != "First journey" || result.Runbooks[1].Title != "Second journey" {
		t.Fatalf("runbooks = %#v", result.Runbooks)
	}
	if len(sources) != 3 || filepath.Base(sources[0]) != "README.md" {
		t.Fatalf("sources = %#v", sources)
	}
	for _, runbook := range result.Runbooks {
		if runbook.ResultPath != filepath.ToSlash(filepath.Join(item.ItemPath, "automation", "results", "latest.md")) {
			t.Fatalf("result path = %q", runbook.ResultPath)
		}
		if runbook.LatestResult == nil || (runbook.LatestResult.Status != "passed" && runbook.LatestResult.Status != "unknown") {
			t.Fatalf("latest result = %#v", runbook.LatestResult)
		}
	}
}

func TestE2ERunbooksSkipsScenarioSymlinkOutsideWorkspace(t *testing.T) {
	service, workspace, item := e2eRunbookTestService(t)
	automation := filepath.Join(workspace.Path, item.ItemPath, "automation")
	if err := os.MkdirAll(automation, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "scenario-01-outside.md")
	writeTestFile(t, outside, "# Outside\n")
	if err := os.Symlink(outside, filepath.Join(automation, "scenario-01-outside.md")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	result, _, err := service.E2ERunbooks(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Runbooks) != 0 {
		t.Fatalf("runbooks = %#v", result.Runbooks)
	}
}

func e2eRunbookTestService(t *testing.T) (*Service, models.WorkspaceConfig, models.ItemDetail) {
	t.Helper()
	root := t.TempDir()
	planPath := filepath.Join("plans", "platform", "PM-036")
	writeTestFile(t, filepath.Join(root, planPath, "plan.yaml"), "plan:\n  e2e-runbook: true\n")
	if output, err := exec.Command("git", "-C", root, "init", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	commit := exec.Command("git", "-C", root, "commit", "--allow-empty", "-m", "seed")
	commit.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	dataDir := t.TempDir()
	reg := registry.New(filepath.Join(dataDir, "workspaces.yaml"), gitadapter.New())
	workspace, err := reg.Create(models.WorkspaceInput{Name: "Test", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	item := models.ItemDetail{ItemSummary: models.ItemSummary{
		ID: "item-pm-036", WorkspaceID: workspace.ID, WorkspaceName: workspace.Name,
		Branch: "main", SourceMode: "working_tree", Editable: true, Scope: "platform",
		Identifier: "PM-036", Title: "E2E Quality Panels", ItemPath: filepath.ToSlash(planPath),
	}}
	index := itemindex.New(filepath.Join(dataDir, "item-index.yaml"))
	if err := index.ReplaceWorkspace(workspace.ID, []models.ItemDetail{item}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	return New(reg, index, nil, nil, gitadapter.New()), workspace, item
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
