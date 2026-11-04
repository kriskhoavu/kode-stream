package harness

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

func writeIn(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func approvePlan(t *testing.T, fixture *pipelineFixture) {
	t.Helper()
	approval := fixture.waiting(t)
	if approval.Gate != domain.GatePlan {
		t.Fatalf("waiting gate = %s", approval.Gate)
	}
	if _, err := fixture.service.Decide(context.Background(), approval.ID, DecideInput{Decision: DecisionApprove, IdempotencyKey: "plan-" + approval.ID}); err != nil {
		t.Fatal(err)
	}
}

func TestImplementStageRevertsOutOfScopeChangesGatesReviewsAndCommits(t *testing.T) {
	ctx := context.Background()
	fixture := newPipeline(t, func(role domain.Role, call int, invocation ports.Invocation) (map[string]any, ports.Result) {
		switch role {
		case domain.RolePlanner:
			writeIn(t, invocation.WorktreePath, "plans/platform/PM-7/plan.yaml", "plan:\n  status: draft\n")
			writeIn(t, invocation.WorktreePath, "stray.txt", "planner wandered\n")
		case domain.RoleCoder:
			writeIn(t, invocation.WorktreePath, "internal/export/export.go", "package export\n")
			writeIn(t, invocation.WorktreePath, ".github/workflows/evil.yml", "on: push\n")
			writeIn(t, invocation.WorktreePath, "docs/notes.md", "outside the scope\n")
			if !strings.Contains(invocation.Brief, "`internal/`") {
				t.Errorf("coder brief lacks its scope:\n%s", invocation.Brief)
			}
		}
		return planReport(role), ports.Result{Turns: 2, CostUSD: 0.1}
	}, func(settings *domain.Settings) {
		settings.Workspaces[0].Scope = []string{"internal/"}
		settings.Workspaces[0].VerificationCommands = []string{"test -f internal/export/export.go"}
	})
	fixture.settle(t)
	view := fixture.run(t)
	if _, err := os.Stat(filepath.Join(view.WorktreePath, "stray.txt")); !os.IsNotExist(err) {
		t.Fatal("the planner's out-of-scope file was kept")
	}
	approvePlan(t, fixture)
	fixture.settle(t)

	view = fixture.run(t)
	var build StageView
	for _, stage := range view.Stages {
		if stage.Kind == domain.StageImplement {
			build = stage
		}
	}
	if build.State != domain.StagePassed || len(build.Rounds) != 1 || build.Rounds[0].Verdict != domain.VerdictApprove || build.Advisory == 0 {
		t.Fatalf("build = %#v", build.Stage)
	}
	if last := view.Stages[len(view.Stages)-1]; last.Kind != domain.StageQA || last.State != domain.StagePending || view.CurrentStageKind != domain.StageQA {
		t.Fatalf("after build = %#v", last.Stage)
	}
	coders, _ := fixture.repository.ListAgentRuns(ctx, domain.AgentRunFilter{StageID: build.ID})
	var coder, reviewer domain.AgentRun
	for _, agentRun := range coders {
		if agentRun.Role == domain.RoleCoder {
			coder = agentRun
		} else {
			reviewer = agentRun
		}
	}
	if strings.Join(coder.OutOfScope, ",") != ".github/workflows/evil.yml,docs/notes.md" || coder.HarnessGate == nil || !coder.HarnessGate.Passed || reviewer.Commit == "" {
		t.Fatalf("coder = %#v reviewer commit = %q", coder, reviewer.Commit)
	}
	for _, gone := range []string{".github/workflows/evil.yml", "docs/notes.md"} {
		if _, err := os.Stat(filepath.Join(view.WorktreePath, gone)); !os.IsNotExist(err) {
			t.Fatalf("%s survived the revert", gone)
		}
	}
	log := gitOutput(t, view.WorktreePath, "log", "--format=%s", view.BaseCommit+"..HEAD")
	if log != "PM-7: Add export\nPM-7: Plan Add export\n" {
		t.Fatalf("branch log = %q", log)
	}
	files := gitOutput(t, view.WorktreePath, "show", "--name-only", "--format=", "HEAD")
	if strings.TrimSpace(files) != "internal/export/export.go" {
		t.Fatalf("build commit files = %q", files)
	}
	if main := strings.TrimSpace(gitOutput(t, fixture.workspace.Path, "rev-parse", "main")); main != view.BaseCommit {
		t.Fatal("the baseline branch moved")
	}
}

func TestAFailingGateSendsTheBuilderBackWithTheLogAndBlocksAfterThreeRounds(t *testing.T) {
	fixture := newPipeline(t, happyScript, func(settings *domain.Settings) {
		settings.Workspaces[0].VerificationCommands = []string{"echo compiling; echo 'FAIL: export_test.go:12'; exit 3"}
	})
	fixture.settle(t)
	approvePlan(t, fixture)
	fixture.settle(t)
	view := fixture.run(t)
	build := view.Stages[len(view.Stages)-1]
	if view.State != domain.RunBlocked || build.Kind != domain.StageImplement || build.State != domain.StageFailed || build.Round != 3 || !strings.Contains(build.FailureReason, "verification still failed") {
		t.Fatalf("run=%s build=%#v", view.State, build.Stage)
	}
	second := fixture.engine.brief(domain.RoleCoder, 1)
	if !strings.Contains(second, "FAIL: export_test.go:12") || !strings.Contains(second, "Verification failed: echo compiling") {
		t.Fatalf("round 2 brief:\n%s", second)
	}
	if fixture.engine.calls[domain.RoleCodeReviewer] != 0 {
		t.Fatal("a failing build reached review")
	}
}

func TestScopeKeepsProtectedPathsOutEvenWithoutAScope(t *testing.T) {
	cases := map[string]bool{"internal/a.go": true, ".git/config": false, ".harness/report.json": false, ".claude/settings.json": false, ".github/workflows/ci.yml": false, ".github/CODEOWNERS": true}
	for path, want := range cases {
		if got := inScope(path, nil); got != want {
			t.Fatalf("inScope(%s, nil) = %v", path, got)
		}
	}
	if inScope("web/app.ts", []string{"internal/"}) || !inScope("internal", []string{"internal/"}) || !inScope("go.mod", []string{"go.mod"}) {
		t.Fatal("prefix scope is wrong")
	}
}
