package harness

import (
	"context"
	"strings"
	"testing"
	"time"

	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

// readyJira turns the workspace's Jira source and its readiness check on.
func readyJira(settings *domain.Settings) {
	on := true
	settings.Workspaces[0].Jira = &domain.JiraSourceSettings{Enabled: true, JQL: "project = PM", CheckReady: &on}
}

// makeReady puts the fixture's ticket in READY with a reporter and an
// assignee, and takes back the start request, so only its status starts it.
func makeReady(t *testing.T, fixture *pipelineFixture, status string) {
	t.Helper()
	ctx := context.Background()
	task, err := fixture.repository.GetTask(ctx, fixture.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	expect := domain.Expect{State: string(task.State), Revision: task.Revision}
	task.StartRequested = false
	task.Source.Status = status
	task.Source.Assignee = "Khoa"
	task.Source.Reporter = &domain.Person{Name: "Bao", Mention: "[~bao]"}
	if _, err := fixture.repository.SaveTask(ctx, task, expect); err != nil {
		t.Fatal(err)
	}
}

// comment adds a comment on the fixture's ticket, as a poll would.
func comment(t *testing.T, fixture *pipelineFixture, id, author, body string) {
	t.Helper()
	ctx := context.Background()
	task, _ := fixture.repository.GetTask(ctx, fixture.task.ID)
	expect := domain.Expect{State: string(task.State), Revision: task.Revision}
	task.Comments = append(task.Comments, domain.Comment{ID: id, Author: author, CreatedAt: time.Now().UTC().Add(time.Second), Body: body})
	if _, err := fixture.repository.SaveTask(ctx, task, expect); err != nil {
		t.Fatal(err)
	}
}

// A READY ticket starts on its own. Its planner finds it says too little, so
// the run waits for the reporter with a draft comment that mentions them; the
// assignee's own comment does not end the wait, the reporter's reply does,
// and the planner then plans from it.
func TestAReadyTicketWaitsForItsReporterAndPlansFromTheReply(t *testing.T) {
	ctx := context.Background()
	fixture := newPipeline(t, func(role domain.Role, call int, invocation ports.Invocation) (map[string]any, ports.Result) {
		if role == domain.RolePlanner && call == 1 {
			if !strings.Contains(invocation.Brief, "Before you plan") || !strings.Contains(invocation.Brief, "questions for Bao") || !strings.Contains(invocation.Brief, "status: needs_info") {
				t.Errorf("first brief lacks the readiness check:\n%s", invocation.Brief)
			}
			report := planReport(role)
			report["status"] = domain.ReportNeedsInfo
			report["summary"] = "The export has no format and no audience."
			report["open"] = []string{"Which format: CSV or XLSX?", "Who downloads it?"}
			return report, ports.Result{Turns: 1, CostUSD: 0.1}
		}
		if role == domain.RolePlanner {
			if !strings.Contains(invocation.Brief, "CSV, for finance.") || !strings.Contains(invocation.Brief, ", on the issue)") {
				t.Errorf("second brief lacks the reply:\n%s", invocation.Brief)
			}
		}
		return planReport(role), ports.Result{Turns: 1, CostUSD: 0.1}
	}, readyJira)
	makeReady(t, fixture, "Ready")
	fixture.settle(t)

	wait := fixture.waiting(t)
	if wait.Gate != domain.GateClarify || wait.Context.Reporter != "Bao" || len(wait.Context.Questions) != 2 || wait.Summary != "The export has no format and no audience." {
		t.Fatalf("approval = %#v", wait)
	}
	if !strings.HasPrefix(wait.Context.Draft, "Hi [~bao],") || !strings.Contains(wait.Context.Draft, "Before we start on PM-7") || !strings.Contains(wait.Context.Draft, "2. Who downloads it?") {
		t.Fatalf("draft = %q", wait.Context.Draft)
	}
	task, _ := fixture.repository.GetTask(ctx, fixture.task.ID)
	if task.State != domain.TaskRunning || task.NeedsYou || task.WaitingFor != "Bao" {
		t.Fatalf("a run waiting for the reporter needs the assignee: %#v", task)
	}

	comment(t, fixture, "c1", "Khoa", "I asked Bao.")
	fixture.service.answerFromSource(ctx)
	if still := fixture.waiting(t); still.ID != wait.ID {
		t.Fatalf("the assignee's comment ended the wait: %#v", still)
	}

	comment(t, fixture, "c2", "Bao", "CSV, for finance.")
	fixture.service.answerFromSource(ctx)
	fixture.settle(t)
	decided, _ := fixture.repository.GetApproval(ctx, wait.ID)
	if decided.State != domain.ApprovalApproved || decided.Channel != domain.ChannelSource || decided.DecidedBy == nil || decided.DecidedBy.DisplayName != "Bao" {
		t.Fatalf("decided = %#v", decided)
	}
	plan := fixture.waiting(t)
	if plan.Gate != domain.GatePlan {
		t.Fatalf("after the reply the run waits at %s", plan.Gate)
	}
	if task, _ := fixture.repository.GetTask(ctx, fixture.task.ID); task.WaitingFor != "" {
		t.Fatalf("the task still waits for %q", task.WaitingFor)
	}
	view := fixture.run(t)
	if answers := view.Stages[0].Answers; len(answers) != 1 || !answers[0].FromSource || answers[0].By != "Bao" {
		t.Fatalf("answers = %#v", answers)
	}
}

// A person may continue without the reporter: their answer ends the
// readiness check, so the planner plans and does not ask again.
func TestAPersonContinuesAReadyTicketWithoutItsReporter(t *testing.T) {
	ctx := context.Background()
	fixture := newPipeline(t, func(role domain.Role, call int, invocation ports.Invocation) (map[string]any, ports.Result) {
		if role == domain.RolePlanner && call == 1 {
			report := planReport(role)
			report["status"] = domain.ReportNeedsInfo
			report["open"] = []string{"Which format?"}
			return report, ports.Result{Turns: 1}
		}
		if role == domain.RolePlanner && strings.Contains(invocation.Brief, "Before you plan") {
			t.Errorf("the readiness check outlived a person's answer:\n%s", invocation.Brief)
		}
		return planReport(role), ports.Result{Turns: 1}
	}, readyJira)
	makeReady(t, fixture, "READY")
	fixture.settle(t)
	wait := fixture.waiting(t)
	if _, err := fixture.service.Decide(ctx, wait.ID, DecideInput{Decision: DecisionApprove, IdempotencyKey: "c-empty", Actor: domain.Actor{DisplayName: "Khoa"}}); !IsInvalid(err) {
		t.Fatalf("continue without a note err = %v", err)
	}
	if _, err := fixture.service.Decide(ctx, wait.ID, DecideInput{Decision: DecisionApprove, Reason: "CSV.", IdempotencyKey: "c-1", Actor: domain.Actor{DisplayName: "Khoa"}}); err != nil {
		t.Fatal(err)
	}
	fixture.settle(t)
	if plan := fixture.waiting(t); plan.Gate != domain.GatePlan {
		t.Fatalf("run waits at %s", plan.Gate)
	}
}

// Dismissing a ticket that waits for its reporter stops its run.
func TestDismissingAReadyTicketStopsItsRun(t *testing.T) {
	ctx := context.Background()
	fixture := newPipeline(t, func(role domain.Role, call int, invocation ports.Invocation) (map[string]any, ports.Result) {
		report := planReport(role)
		report["status"] = domain.ReportNeedsInfo
		return report, ports.Result{Turns: 1}
	}, readyJira)
	makeReady(t, fixture, "READY")
	fixture.settle(t)
	wait := fixture.waiting(t)
	if _, err := fixture.service.Decide(ctx, wait.ID, DecideInput{Decision: DecisionReject, Reason: "Not ours.", IdempotencyKey: "d-1", Actor: domain.Actor{DisplayName: "Khoa"}}); err != nil {
		t.Fatal(err)
	}
	if view := fixture.run(t); view.State != domain.RunCancelled {
		t.Fatalf("run = %s", view.State)
	}
	if task, _ := fixture.repository.GetTask(ctx, fixture.task.ID); task.WaitingFor != "" || task.State != domain.TaskCancelled {
		t.Fatalf("task = %s waiting for %q", task.State, task.WaitingFor)
	}
}

// Only a ticket in a ready status, with the check on, starts on its own.
func TestOnlyAReadyTicketStartsOnItsOwn(t *testing.T) {
	ctx := context.Background()
	off := false
	for name, setup := range map[string]struct {
		status string
		adjust func(*domain.Settings)
	}{
		"another status": {status: "In Progress", adjust: readyJira},
		"check off": {status: "READY", adjust: func(settings *domain.Settings) {
			readyJira(settings)
			settings.Workspaces[0].Jira.CheckReady = &off
		}},
		"check unset": {status: "READY", adjust: func(settings *domain.Settings) {
			settings.Workspaces[0].Jira = &domain.JiraSourceSettings{Enabled: true, JQL: "project = PM"}
		}},
		"no Jira source": {status: "READY", adjust: func(*domain.Settings) {}},
	} {
		fixture := newPipeline(t, func(role domain.Role, call int, invocation ports.Invocation) (map[string]any, ports.Result) {
			return planReport(role), ports.Result{Turns: 1}
		}, setup.adjust)
		makeReady(t, fixture, setup.status)
		fixture.settle(t)
		if task, _ := fixture.repository.GetTask(ctx, fixture.task.ID); task.State != domain.TaskRanked || task.RunID != "" {
			t.Fatalf("%s: task = %s run %q", name, task.State, task.RunID)
		}
	}
}

// After MaxClarifyRounds replies the planner's questions go to a person.
func TestAReadyTicketAsksAPersonAfterTheLastReply(t *testing.T) {
	ctx := context.Background()
	fixture := newPipeline(t, func(role domain.Role, call int, invocation ports.Invocation) (map[string]any, ports.Result) {
		report := planReport(role)
		if role == domain.RolePlanner {
			report["status"] = domain.ReportNeedsInfo
			report["open"] = []string{"Which format?"}
		}
		return report, ports.Result{Turns: 1}
	}, readyJira)
	makeReady(t, fixture, "READY")
	fixture.settle(t)
	for reply := 1; reply <= domain.MaxClarifyRounds; reply++ {
		if wait := fixture.waiting(t); wait.Gate != domain.GateClarify {
			t.Fatalf("reply %d: run waits at %s", reply, wait.Gate)
		}
		comment(t, fixture, "r"+string(rune('0'+reply)), "Bao", "Still thinking.")
		fixture.service.answerFromSource(ctx)
		fixture.settle(t)
	}
	if ask := fixture.waiting(t); ask.Gate != domain.GateQuestion {
		t.Fatalf("after %d replies the run waits at %s", domain.MaxClarifyRounds, ask.Gate)
	}
}

func TestClarifyDraftNamesTheReporterWithoutAMention(t *testing.T) {
	task := domain.Task{Source: domain.Source{Key: "PM-1", Reporter: &domain.Person{Name: "Bao"}}}
	draft := clarifyDraft(task, "Unclear.", []string{"Why?"})
	if !strings.HasPrefix(draft, "Hi @Bao,") || !strings.Contains(draft, "1. Why?") {
		t.Fatalf("draft = %q", draft)
	}
	if draft := clarifyDraft(domain.Task{Source: domain.Source{Key: "PM-1"}}, "Unclear.", nil); !strings.HasPrefix(draft, "Hi,") || strings.Contains(draft, "Questions:") {
		t.Fatalf("draft without a reporter = %q", draft)
	}
}
