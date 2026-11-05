package harness

import (
	"context"
	"strings"
	"testing"

	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

// A planner that lists open questions stops its round at a question gate;
// the answer sends the planner out again with the questions and the answer
// in its brief, and the plan then reaches review as usual.
func TestAPlannerWithOpenQuestionsStopsToAskAndContinuesFromTheAnswer(t *testing.T) {
	ctx := context.Background()
	fixture := newPipeline(t, func(role domain.Role, call int, invocation ports.Invocation) (map[string]any, ports.Result) {
		if role == domain.RolePlanner && call == 1 {
			report := planReport(role)
			report["open"] = []string{"Should the export include archived offers?", "Is CSV enough, or XLSX too?"}
			return report, ports.Result{Turns: 1, CostUSD: 0.2}
		}
		if role == domain.RolePlanner && strings.Contains(invocation.Brief, "A person answered") {
			report := planReport(role)
			report["summary"] = "Planned with the answers."
			return report, ports.Result{Turns: 1, CostUSD: 0.2}
		}
		return planReport(role), ports.Result{Turns: 1, CostUSD: 0.1}
	})
	fixture.settle(t)
	ask := fixture.waiting(t)
	if ask.Gate != domain.GateQuestion || len(ask.Context.Questions) != 2 || ask.Context.Role != domain.RolePlanner || !strings.Contains(ask.Summary, "stopped to ask") {
		t.Fatalf("approval = %#v", ask)
	}
	view := fixture.run(t)
	plan := view.Stages[0]
	if view.State != domain.RunRunning || plan.State != domain.StageAwaitingApproval || plan.Round != 1 {
		t.Fatalf("run = %s plan = %#v", view.State, plan.Stage)
	}
	task, _ := fixture.repository.GetTask(ctx, fixture.task.ID)
	if task.State != domain.TaskRunning || !task.NeedsYou {
		t.Fatalf("task = %#v", task)
	}
	if _, err := fixture.service.Decide(ctx, ask.ID, DecideInput{Decision: DecisionApprove, IdempotencyKey: "q-empty"}); !IsInvalid(err) {
		t.Fatalf("answer without text err = %v", err)
	}
	if _, err := fixture.service.Decide(ctx, ask.ID, DecideInput{Decision: DecisionApprove, Reason: "Archived too. CSV is enough.", IdempotencyKey: "q-1", Actor: domain.Actor{DisplayName: "Anna"}}); err != nil {
		t.Fatal(err)
	}
	fixture.settle(t)
	second := fixture.engine.brief(domain.RolePlanner, 1)
	if !strings.Contains(second, "A person answered") || !strings.Contains(second, "Q: Should the export include archived offers?") || !strings.Contains(second, "A (Anna): Archived too. CSV is enough.") {
		t.Fatalf("second brief:\n%s", second)
	}
	view = fixture.run(t)
	plan = view.Stages[0]
	if plan.State != domain.StagePassed || len(plan.Answers) != 1 || plan.Answers[0].By != "Anna" {
		t.Fatalf("plan after answer = %#v", plan.Stage)
	}
	gate := fixture.waiting(t)
	if gate.Gate != domain.GatePlan {
		t.Fatalf("after the answer the run waits on %s, want the plan gate", gate.Gate)
	}
}

// With the gate off for the task, open questions ride along to the plan
// gate and a blocked report blocks the run instead of asking.
func TestWithTheQuestionGateOffQuestionsReachThePlanGateAndBlockedBlocks(t *testing.T) {
	ctx := context.Background()
	no := false
	fixture := newPipeline(t, func(role domain.Role, call int, invocation ports.Invocation) (map[string]any, ports.Result) {
		report := planReport(role)
		if role == domain.RolePlanner {
			report["open"] = []string{"Archived offers too?"}
		}
		return report, ports.Result{Turns: 1, CostUSD: 0.1}
	}, func(settings *domain.Settings) { settings.AskQuestions = &no })
	fixture.settle(t)
	gate := fixture.waiting(t)
	if gate.Gate != domain.GatePlan || len(gate.Context.Questions) != 1 {
		t.Fatalf("approval = %#v", gate)
	}

	yes := true
	blocked := newPipeline(t, func(role domain.Role, call int, invocation ports.Invocation) (map[string]any, ports.Result) {
		report := planReport(role)
		if role == domain.RolePlanner {
			report["status"], report["summary"] = "blocked", "The repository has no export module to extend."
		}
		return report, ports.Result{Turns: 1, CostUSD: 0.1}
	}, func(settings *domain.Settings) {
		settings.AskQuestions = &yes
		settings.Workspaces[0].AskQuestions = &no
	})
	blocked.settle(t)
	view := blocked.run(t)
	if view.State != domain.RunBlocked || !strings.Contains(view.FailureReason, "no export module") {
		t.Fatalf("run = %s %q", view.State, view.FailureReason)
	}
	task, _ := blocked.repository.GetTask(ctx, blocked.task.ID)
	if task.State != domain.TaskBlocked {
		t.Fatalf("task = %s", task.State)
	}
	settings := blocked.service.settingsNow(ctx)
	if settings.QuestionGate(task) {
		t.Fatal("the workspace's no did not win over the settings' yes")
	}
	task.AskQuestions = &yes
	if !settings.QuestionGate(task) {
		t.Fatal("the task's yes did not win over the workspace's no")
	}
}
