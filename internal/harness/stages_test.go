package harness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

type fakePulls struct {
	mu       sync.Mutex
	failures int
	requests []ports.PullRequest
}

func (f *fakePulls) ID() string { return "fake" }
func (f *fakePulls) Supports(models.WorkspaceConfig, domain.WorkspaceSettings) bool {
	return true
}
func (f *fakePulls) CreateDraft(_ context.Context, request ports.PullRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, request)
	if f.failures > 0 {
		f.failures--
		return "", errors.New("provider unavailable")
	}
	return "https://example.test/pull/1", nil
}

type auditLog struct {
	mu     sync.Mutex
	events []models.AuditEvent
}

func (a *auditLog) Append(event models.AuditEvent) (models.AuditEvent, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, event)
	return event, nil
}

func withOrigin(t *testing.T, fixture *pipelineFixture) string {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, filepath.Dir(origin), "init", "-q", "--bare", "-b", "main", origin)
	runGit(t, fixture.workspace.Path, "remote", "add", "origin", origin)
	runGit(t, fixture.workspace.Path, "push", "-q", "origin", "main")
	return origin
}

func failOnRun(t *testing.T, n int) string {
	counter := filepath.Join(t.TempDir(), "runs")
	return "c=$(cat " + counter + " 2>/dev/null || echo 0); c=$((c+1)); echo $c > " + counter + "; if [ $c -eq " + string(rune('0'+n)) + " ]; then echo 'export.go is broken'; exit 1; fi"
}

func TestARunGoesFromBuildThroughTestsToADraftPullRequest(t *testing.T) {
	ctx := context.Background()
	draftRetryDelay = 0
	defer func() { draftRetryDelay = time.Minute }()
	// Gate runs: 1 the builder's, 2 QA's first (fails), 3 the tester's, 4 and 5 QA's second try.
	command := failOnRun(t, 2)
	fixture := newPipeline(t, func(role domain.Role, call int, invocation ports.Invocation) (map[string]any, ports.Result) {
		switch role {
		case domain.RoleCoder:
			writeIn(t, invocation.WorktreePath, "internal/export.go", "package export\n")
		case domain.RoleQAFixer:
			if !strings.Contains(invocation.Brief, "export.go is broken") {
				t.Errorf("tester brief lacks the log:\n%s", invocation.Brief)
			}
			writeIn(t, invocation.WorktreePath, "internal/export_test.go", "package export\n")
		}
		return planReport(role), ports.Result{Turns: 1, CostUSD: 0.1}
	}, func(settings *domain.Settings) {
		settings.Workspaces[0].VerificationCommands = []string{command}
	})
	origin := withOrigin(t, fixture)
	pulls := &fakePulls{failures: 1}
	audit := &auditLog{}
	fixture.service.pulls, fixture.service.audit = []ports.PullRequests{pulls}, audit
	fixture.settle(t)
	approvePlan(t, fixture)
	fixture.settle(t)

	view := fixture.run(t)
	if view.State != domain.RunPassed || view.PRURL != "https://example.test/pull/1" || !view.WorktreeRemoved || view.FinishedAt == nil {
		t.Fatalf("run = %#v", view.Run)
	}
	var qa, draft StageView
	for _, stage := range view.Stages {
		switch stage.Kind {
		case domain.StageQA:
			qa = stage
		case domain.StageDraftPR:
			draft = stage
		}
	}
	if qa.State != domain.StagePassed || qa.Round != 1 || qa.LastGate == nil || !qa.LastGate.Passed || len(qa.AgentRuns) != 1 || qa.AgentRuns[0].Role != domain.RoleQAFixer {
		t.Fatalf("qa = %#v", qa.Stage)
	}
	remote := "feature/" + strings.TrimPrefix(view.Branch, "harness/")
	if draft.State != domain.StagePassed || draft.Attempt != 2 || len(pulls.requests) != 2 || pulls.requests[0].Base != "main" || pulls.requests[0].Branch != remote || !strings.HasPrefix(pulls.requests[0].Title, "PM-7: ") {
		t.Fatalf("draft = %#v requests=%d", draft.Stage, len(pulls.requests))
	}
	task, _ := fixture.repository.GetTask(ctx, fixture.task.ID)
	if task.State != domain.TaskDone || task.NeedsYou {
		t.Fatalf("task = %#v", task)
	}
	if _, err := os.Stat(view.WorktreePath); !os.IsNotExist(err) {
		t.Fatal("the worktree was kept after the pull request")
	}
	pushed := gitOutput(t, origin, "log", "--format=%s", remote)
	if !strings.HasPrefix(pushed, "PM-7: Fix the tests of Add export\nPM-7: Add export\nbase\n") {
		t.Fatalf("pushed branch log = %q", pushed)
	}
	operations := map[string]int{}
	for _, event := range audit.events {
		operations[event.Operation+":"+string(event.Status)]++
	}
	if operations["harness.push:success"] != 2 || operations["harness.pull_request:failed"] != 1 || operations["harness.pull_request:success"] != 1 || operations["harness.commit:success"] != 1 {
		t.Fatalf("audit = %v", operations)
	}
}

func TestTestsThatKeepFailingBlockTheRunAndAPRApprovalGatesThePush(t *testing.T) {
	failing := newPipeline(t, happyScript, func(settings *domain.Settings) {
		settings.Workspaces[0].VerificationCommands = []string{"test ! -f internal/export.go || { echo still broken; exit 1; }"}
	})
	failing.engine.script = func(role domain.Role, call int, invocation ports.Invocation) (map[string]any, ports.Result) {
		if role == domain.RoleCodeReviewer {
			writeIn(t, invocation.WorktreePath, "internal/export.go", "package export\n")
		}
		return planReport(role), ports.Result{}
	}
	failing.settle(t)
	approvePlan(t, failing)
	failing.settle(t)
	view := failing.run(t)
	last := view.Stages[len(view.Stages)-1]
	if view.State != domain.RunBlocked || last.Kind != domain.StageQA || last.Round != 2 || !strings.Contains(last.FailureReason, "2 fix rounds") {
		t.Fatalf("run=%s qa=%#v", view.State, last.Stage)
	}

}

func reviewing(t *testing.T) (*pipelineFixture, *fakePulls, string) {
	t.Helper()
	fixture := newPipeline(t, happyScript, func(settings *domain.Settings) { settings.RequireReview = true })
	origin := withOrigin(t, fixture)
	pulls := &fakePulls{}
	fixture.service.pulls = []ports.PullRequests{pulls}
	fixture.settle(t)
	approvePlan(t, fixture)
	fixture.settle(t)
	if approval := fixture.waiting(t); approval.Gate != domain.GateReview || len(pulls.requests) != 0 {
		t.Fatalf("approval = %#v requests=%d", approval, len(pulls.requests))
	}
	return fixture, pulls, origin
}

func review(t *testing.T, fixture *pipelineFixture, decision Decision, reason string) {
	t.Helper()
	approval := fixture.waiting(t)
	if approval.Gate != domain.GateReview {
		t.Fatalf("waiting gate = %s", approval.Gate)
	}
	if _, err := fixture.service.Decide(context.Background(), approval.ID, DecideInput{Decision: decision, Reason: reason, IdempotencyKey: "review-" + approval.ID}); err != nil {
		t.Fatal(err)
	}
	fixture.settle(t)
}

func kinds(view RunView) []string {
	var out []string
	for _, stage := range view.Stages {
		out = append(out, string(stage.Kind))
	}
	return out
}

func TestReviewHandsOffWithoutTestingAgainWhenNothingChanged(t *testing.T) {
	fixture, pulls, _ := reviewing(t)
	review(t, fixture, DecisionApprove, "")
	view := fixture.run(t)
	if view.State != domain.RunPassed || len(pulls.requests) != 1 {
		t.Fatalf("run=%s requests=%d", view.State, len(pulls.requests))
	}
	if got := strings.Join(kinds(view), ","); got != "plan,plan_approval,implement,qa,review,draft_pr" {
		t.Fatalf("stages = %s", got)
	}
}

func TestReviewCommitsThePersonsChangesAndTestsThemBeforeHandingOff(t *testing.T) {
	fixture, pulls, origin := reviewing(t)
	worktree := fixture.run(t).WorktreePath
	writeIn(t, worktree, "internal/reviewed.go", "package internal\n")
	review(t, fixture, DecisionApprove, "")
	view := fixture.run(t)
	if view.State != domain.RunPassed || len(pulls.requests) != 1 {
		t.Fatalf("run=%s requests=%d", view.State, len(pulls.requests))
	}
	if got := strings.Join(kinds(view), ","); got != "plan,plan_approval,implement,qa,review,qa,draft_pr" {
		t.Fatalf("stages = %s", got)
	}
	if log := gitOutput(t, origin, "log", "--format=%s", "--all"); !strings.Contains(log, "Apply the review of") {
		t.Fatalf("branch log = %s", log)
	}
}

func TestReviewSendsTheChangeBackToBuildOrPlanWithTheReviewersWords(t *testing.T) {
	fixture, pulls, _ := reviewing(t)
	review(t, fixture, DecisionReject, "Rename the flag")
	view := fixture.run(t)
	if got := strings.Join(kinds(view), ","); got != "plan,plan_approval,implement,qa,review,implement,qa,review" {
		t.Fatalf("stages after Build = %s", got)
	}
	again := view.Stages[5]
	if len(again.Findings) == 0 || again.Findings[0].Detail != "Rename the flag" || again.Findings[0].Severity != "blocking" {
		t.Fatalf("findings = %#v", again.Findings)
	}

	review(t, fixture, DecisionReplan, "Split the export")
	approvePlan(t, fixture)
	fixture.settle(t)
	view = fixture.run(t)
	if got := strings.Join(kinds(view), ","); got != "plan,plan_approval,implement,qa,review,implement,qa,review,plan,plan_approval,implement,qa,review" {
		t.Fatalf("stages after Plan = %s", got)
	}
	if len(pulls.requests) != 0 {
		t.Fatalf("requests = %d", len(pulls.requests))
	}
	approval := fixture.waiting(t)
	if _, err := fixture.service.Decide(context.Background(), approval.ID, DecideInput{Decision: DecisionReplan, IdempotencyKey: "replan-empty"}); err == nil {
		t.Fatal("a replan without a reason was accepted")
	}
}

func TestJanitorRemovesExpiredWorktreesOnly(t *testing.T) {
	ctx := context.Background()
	fixture := newPipeline(t, happyScript)
	fixture.settle(t)
	view := fixture.run(t)
	if _, err := fixture.service.CancelRun(ctx, view.ID, "", nil, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.janitorCycle(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(view.WorktreePath); err != nil {
		t.Fatal("a fresh cancelled run lost its worktree")
	}
	fixture.service.now = func() time.Time { return time.Now().UTC().Add(4 * 24 * time.Hour) }
	if err := fixture.service.janitorCycle(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(view.WorktreePath); !os.IsNotExist(err) {
		t.Fatal("an expired worktree was kept")
	}
	if run, _ := fixture.repository.GetRun(ctx, view.ID); !run.WorktreeRemoved {
		t.Fatal("the run does not record the removal")
	}
	branches := gitOutput(t, fixture.workspace.Path, "branch", "--list", "harness/*")
	if !strings.Contains(branches, view.Branch) {
		t.Fatal("the janitor deleted an unpushed branch")
	}
}

func TestACheckerJudgesTheChangeAgainstWhatAPersonAskedAtReview(t *testing.T) {
	var briefs []string
	fixture := newPipeline(t, func(role domain.Role, call int, invocation ports.Invocation) (map[string]any, ports.Result) {
		if role == domain.RoleCodeReviewer {
			briefs = append(briefs, invocation.Brief)
		}
		return happyScript(role, call, invocation)
	}, func(settings *domain.Settings) { settings.RequireReview = true })
	withOrigin(t, fixture)
	fixture.service.pulls = []ports.PullRequests{&fakePulls{}}
	fixture.settle(t)
	approvePlan(t, fixture)
	fixture.settle(t)
	review(t, fixture, DecisionReject, "Also print the year")
	if len(briefs) != 2 {
		t.Fatalf("code reviews = %d", len(briefs))
	}
	if strings.Contains(briefs[0], "A person asked for these changes") {
		t.Fatalf("the first check had no request:\n%s", briefs[0])
	}
	for _, want := range []string{"A person asked for these changes on top of the issue", "Asked in Review by", "Also print the year", "do not ask to undo them"} {
		if !strings.Contains(briefs[1], want) {
			t.Fatalf("second check lacks %q:\n%s", want, briefs[1])
		}
	}
}
