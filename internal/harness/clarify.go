package harness

import (
	"context"
	"fmt"
	"log"
	"strings"

	"kode-stream/internal/harness/domain"
)

func readinessOpen(settings domain.Settings, task domain.Task, stage domain.Stage) bool {
	if stage.Kind != domain.StagePlan || !settings.ReadyCheck(task) {
		return false
	}
	replies := 0
	for _, answer := range stage.Answers {
		if answer.Role != domain.RolePlanner {
			continue
		}
		if !answer.FromSource {
			return false
		}
		replies++
	}
	return replies < domain.MaxClarifyRounds
}

func (st *step) needsInfo(agentRun domain.AgentRun) {
	if readinessOpen(st.settings, st.task, st.stage) {
		st.clarify(agentRun)
		return
	}
	report := *agentRun.Report
	report.Status = "blocked"
	agentRun.Report = &report
	if !st.stopsToAsk(agentRun) {
		st.block("the planner found the ticket does not say enough to plan from: " + nonEmpty(report.Summary, "it gave no reason") + ".")
	}
}

func (st *step) clarify(agentRun domain.AgentRun) {
	report := agentRun.Report
	asked := questions(report)
	problem := boundedText(nonEmpty(report.Summary, "The ticket does not say enough to plan from."), domain.MaxSummaryChars)
	reporter := ""
	if st.task.Source.Reporter != nil {
		reporter = st.task.Source.Reporter.Name
	}
	st.stage.State = domain.StageAwaitingApproval
	approval := domain.Approval{ID: domain.NewID("apr"), RunID: st.run.ID, TaskID: st.task.ID, StageID: st.stage.ID, SourceKey: st.task.Source.Key, Title: st.task.Title, Gate: domain.GateClarify, State: domain.ApprovalRequested,
		Summary: problem, Context: domain.ApprovalContext{CostSoFarUSD: roundCents(st.run.Totals.CostUSD), Round: st.stage.Round, Questions: asked, Role: domain.RolePlanner, Reporter: reporter, Draft: clarifyDraft(st.task, problem, asked)}, RequestedAt: st.now}
	st.approvals = append(st.approvals, approval)
	st.task.NeedsYou = false
	st.task.WaitingFor = nonEmpty(reporter, "the reporter")
	st.event(domain.EventStageRound, st.stage.ID, map[string]any{"kind": st.stage.Kind, "round": st.stage.Round, "needsInfo": len(asked), "reporter": reporter})
	st.events = append(st.events, domain.Event{Kind: domain.EventApprovalRequested, TaskID: st.task.ID, RunID: st.run.ID, StageID: st.stage.ID, ApprovalID: approval.ID, Payload: payload(map[string]any{"gate": approval.Gate, "summary": approval.Summary, "sourceKey": st.task.Source.Key, "title": st.task.Title, "questions": asked, "reporter": reporter})})
}

// The harness never posts this comment. A person may.
func clarifyDraft(task domain.Task, problem string, asked []string) string {
	var b strings.Builder
	greeting := "Hi"
	if reporter := task.Source.Reporter; reporter != nil {
		if reporter.Mention != "" {
			greeting += " " + reporter.Mention
		} else if reporter.Name != "" {
			greeting += " @" + reporter.Name
		}
	}
	fmt.Fprintf(&b, "%s,\n\nBefore we start on %s, we need a few details.\n\nWhat we understand so far: %s\n", greeting, task.Source.Key, problem)
	if len(asked) > 0 {
		b.WriteString("\nQuestions:\n")
		for i, question := range asked {
			fmt.Fprintf(&b, "%d. %s\n", i+1, question)
		}
	}
	b.WriteString("\nThank you!")
	return b.String()
}

func (s *Service) applyClarify(st *step, approval domain.Approval, input DecideInput) error {
	if st.stage.State != domain.StageAwaitingApproval || st.run.State != domain.RunRunning {
		return domain.IllegalTransitionError{Entity: "stage", From: string(st.stage.State), To: "clarified"}
	}
	if input.Decision != DecisionApprove {
		return s.cancelInTx(st, "Dismissed while waiting for the reporter: "+input.Reason)
	}
	fromSource := input.Channel == domain.ChannelSource
	if !fromSource && input.Reason == "" {
		return domain.Invalid("say what the planner should know to continue")
	}
	st.task.NeedsYou, st.task.WaitingFor = false, ""
	st.stage.State = domain.StageRunning
	st.stage.Answers = append(st.stage.Answers, domain.Answer{Role: domain.RolePlanner, Round: st.stage.Round, Questions: approval.Context.Questions, Answer: input.Reason, By: nonEmptyName(input.Actor), FromSource: fromSource})
	st.enqueue(domain.RolePlanner, false)
	st.event(domain.EventStageRound, st.stage.ID, map[string]any{"kind": st.stage.Kind, "round": st.stage.Round, "clarified": true, "fromSource": fromSource})
	return st.save()
}

func (s *Service) answerFromSource(ctx context.Context) {
	approvals, err := s.repository.ListApprovals(ctx, domain.ApprovalFilter{States: []domain.ApprovalState{domain.ApprovalRequested}, Limit: 1000})
	if err != nil {
		log.Printf("harness: list the approvals waiting for a reporter: %v", err)
		return
	}
	for _, approval := range approvals {
		if approval.Gate != domain.GateClarify {
			continue
		}
		task, err := s.repository.GetTask(ctx, approval.TaskID)
		if err != nil {
			continue
		}
		reply, ok := replyAfter(task, approval)
		if !ok {
			continue
		}
		input := DecideInput{Decision: DecisionApprove, Reason: boundedText(nonEmpty(reply.Author, "Someone")+" commented on "+task.Source.Key+". Read the ticket's newest comments.", domain.MaxSummaryChars),
			IdempotencyKey: "source-reply-" + approval.ID, Actor: domain.Actor{ActorID: "source", DisplayName: nonEmpty(reply.Author, "the ticket")}, Channel: domain.ChannelSource}
		if _, err := s.Decide(ctx, approval.ID, input); err != nil {
			log.Printf("harness: continue %s after a reply on the ticket: %v", task.Source.Key, err)
		}
	}
}

func replyAfter(task domain.Task, approval domain.Approval) (domain.Comment, bool) {
	for _, comment := range task.Comments {
		if !comment.CreatedAt.After(approval.RequestedAt) {
			continue
		}
		if task.Source.Assignee != "" && strings.EqualFold(strings.TrimSpace(comment.Author), strings.TrimSpace(task.Source.Assignee)) {
			continue
		}
		return comment, true
	}
	return domain.Comment{}, false
}
