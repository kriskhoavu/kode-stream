package harness

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"kode-stream/internal/harness/application/brief"
	"kode-stream/internal/harness/domain"
)

// step holds one transaction's view of a run, its current stage and its task,
// and saves every change together: stage, new stages, run, task, queued agent
// runs, approvals and events.
type step struct {
	s        *Service
	ctx      context.Context
	tx       domain.Repository
	record   func(domain.Event) error
	settings domain.Settings
	now      time.Time

	run         domain.Run
	runExpect   domain.Expect
	stage       domain.Stage
	stageExpect domain.Expect
	task        domain.Task
	taskExpect  domain.Expect

	newStages []domain.Stage
	agentRuns []domain.AgentRun
	approvals []domain.Approval
	events    []domain.Event
}

func (s *Service) loadStep(ctx context.Context, tx domain.Repository, record func(domain.Event) error, settings domain.Settings, runID, stageID string) (*step, error) {
	run, err := tx.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	stage, err := tx.GetStage(ctx, stageID)
	if err != nil {
		return nil, err
	}
	task, err := tx.GetTask(ctx, run.TaskID)
	if err != nil {
		return nil, err
	}
	return &step{s: s, ctx: ctx, tx: tx, record: record, settings: settings, now: s.now(),
		run: run, runExpect: domain.Expect{State: string(run.State), Revision: run.Revision},
		stage: stage, stageExpect: domain.Expect{State: string(stage.State), Revision: stage.Revision},
		task: task, taskExpect: domain.Expect{State: string(task.State), Revision: task.Revision}}, nil
}

func (st *step) event(kind domain.EventKind, stageID string, fields map[string]any) {
	st.events = append(st.events, domain.Event{Kind: kind, TaskID: st.task.ID, RunID: st.run.ID, StageID: stageID, Payload: payload(fields)})
}

func (st *step) save() error {
	if _, err := st.tx.SaveStage(st.ctx, st.stage, st.stageExpect); err != nil {
		return err
	}
	for _, stage := range st.newStages {
		if err := st.tx.InsertStage(st.ctx, stage); err != nil {
			return err
		}
	}
	st.run.Totals.CostUSD = roundCents(st.run.Totals.CostUSD)
	if _, err := st.tx.SaveRun(st.ctx, st.run, st.runExpect); err != nil {
		return err
	}
	st.task.UpdatedAt = st.now
	if _, err := st.tx.SaveTask(st.ctx, st.task, st.taskExpect); err != nil {
		return err
	}
	for _, agentRun := range st.agentRuns {
		if err := st.tx.InsertAgentRun(st.ctx, agentRun); err != nil {
			return err
		}
	}
	for _, approval := range st.approvals {
		if err := st.tx.InsertApproval(st.ctx, approval); err != nil {
			return err
		}
	}
	for _, event := range st.events {
		if err := st.record(event); err != nil {
			return err
		}
	}
	return nil
}

func roundCents(value float64) float64 {
	return float64(int64(value*10000+0.5)) / 10000
}

// addCost charges one agent run to its stage and run.
func (st *step) addCost(agentRun domain.AgentRun) {
	st.stage.Turns += agentRun.Turns
	st.stage.DurationMs += agentRun.DurationMs
	st.stage.CostUSD = roundCents(st.stage.CostUSD + agentRun.CostUSD)
	st.run.Totals.Turns += agentRun.Turns
	st.run.Totals.DurationMs += agentRun.DurationMs
	st.run.Totals.CostUSD += agentRun.CostUSD
}

// enqueue queues an agent run of role on the current stage and round.
func (st *step) enqueue(role domain.Role, reminder bool) {
	agentRun := st.s.newAgentRun(st.run, st.stage, role, st.settings, reminder)
	st.stage.Engine, st.stage.Model = agentRun.Engine, agentRun.Model
	st.agentRuns = append(st.agentRuns, agentRun)
}

func (st *step) round(number int) *domain.Round {
	for i := range st.stage.Rounds {
		if st.stage.Rounds[i].Number == number {
			return &st.stage.Rounds[i]
		}
	}
	st.stage.Rounds = append(st.stage.Rounds, domain.Round{Number: number})
	return &st.stage.Rounds[len(st.stage.Rounds)-1]
}

// retryOrBlock retries a failed agent run's role while the stage has attempts
// left, and blocks the run when it has none.
func (st *step) retryOrBlock(role domain.Role, reason string) {
	if st.stage.Attempt < st.stage.MaxAttempts {
		st.stage.Attempt++
		st.enqueue(role, false)
		st.event(domain.EventStageRound, st.stage.ID, map[string]any{"kind": st.stage.Kind, "round": st.stage.Round, "attempt": st.stage.Attempt, "retry": reason})
		return
	}
	st.block(reason)
}

// block fails the current stage, parks the run and asks a person whether to
// resume it.
func (st *step) block(reason string) {
	st.stage.State = domain.StageFailed
	st.stage.FailureReason = boundedText(reason, 600)
	st.stage.FinishedAt = &st.now
	st.run.State = domain.RunBlocked
	st.run.FailureReason = st.stage.FailureReason
	st.task.State = domain.TaskBlocked
	st.task.NeedsYou = true
	st.event(domain.EventStageFinished, st.stage.ID, map[string]any{"kind": st.stage.Kind, "state": st.stage.State, "reason": st.stage.FailureReason})
	st.event(domain.EventRunBlocked, st.stage.ID, map[string]any{"reason": st.stage.FailureReason, "stage": st.stage.Kind})
	st.request(domain.GateResume, st.stage, "Stopped at "+stageWord(st.stage.Kind)+": "+st.stage.FailureReason+" Resume to try the stage again.")
}

// request asks a person to decide gate.
func (st *step) request(gate domain.GateKind, stage domain.Stage, summary string) {
	approval := domain.Approval{ID: domain.NewID("apr"), RunID: st.run.ID, TaskID: st.task.ID, StageID: stage.ID, SourceKey: st.task.Source.Key, Title: st.task.Title, Gate: gate, State: domain.ApprovalRequested,
		Summary: boundedText(summary, domain.MaxSummaryChars), Context: domain.ApprovalContext{Difficulty: st.run.Difficulty, CostSoFarUSD: roundCents(st.run.Totals.CostUSD), Round: stage.Round}, RequestedAt: st.now}
	if report := st.lastReport(domain.RolePlanner); report != nil {
		approval.Context.Subtasks = len(report.Subtasks)
		if gate == domain.GatePlan {
			approval.Context.Questions = questions(report)
		}
	}
	st.approvals = append(st.approvals, approval)
	st.task.NeedsYou = true
	st.events = append(st.events, domain.Event{Kind: domain.EventApprovalRequested, TaskID: st.task.ID, RunID: st.run.ID, StageID: stage.ID, ApprovalID: approval.ID, Payload: payload(map[string]any{"gate": gate, "summary": approval.Summary, "sourceKey": st.task.Source.Key, "title": st.task.Title})})
}

// questions is what a report leaves for a person to settle, bounded.
func questions(report *domain.Report) []string {
	if report == nil {
		return nil
	}
	var out []string
	for _, question := range report.Open {
		if question = strings.TrimSpace(question); question != "" {
			out = append(out, boundedText(question, 400))
		}
		if len(out) == domain.MaxQuestions {
			break
		}
	}
	return out
}

// ask stops the stage on an agent's own questions and asks a person. The
// stage waits in its round; the answer sends the same role out again with
// the questions and the answer in its brief.
func (st *step) ask(agentRun domain.AgentRun, asked []string) {
	st.stage.State = domain.StageAwaitingApproval
	summary := "The " + roleWord(agentRun.Role) + " stopped to ask you before going on."
	if report := agentRun.Report; report != nil && report.Status == "blocked" && report.Summary != "" {
		summary = "The " + roleWord(agentRun.Role) + " could not go on: " + report.Summary
	}
	approval := domain.Approval{ID: domain.NewID("apr"), RunID: st.run.ID, TaskID: st.task.ID, StageID: st.stage.ID, SourceKey: st.task.Source.Key, Title: st.task.Title, Gate: domain.GateQuestion, State: domain.ApprovalRequested,
		Summary: boundedText(summary, domain.MaxSummaryChars), Context: domain.ApprovalContext{Difficulty: st.run.Difficulty, CostSoFarUSD: roundCents(st.run.Totals.CostUSD), Round: st.stage.Round, Questions: asked, Role: agentRun.Role}, RequestedAt: st.now}
	st.approvals = append(st.approvals, approval)
	st.task.NeedsYou = true
	st.event(domain.EventStageRound, st.stage.ID, map[string]any{"kind": st.stage.Kind, "round": st.stage.Round, "asked": len(asked), "role": agentRun.Role})
	st.events = append(st.events, domain.Event{Kind: domain.EventApprovalRequested, TaskID: st.task.ID, RunID: st.run.ID, StageID: st.stage.ID, ApprovalID: approval.ID, Payload: payload(map[string]any{"gate": approval.Gate, "summary": approval.Summary, "sourceKey": st.task.Source.Key, "title": st.task.Title, "questions": asked})})
}

// stopsToAsk decides what a producer's report that is blocked or asks
// questions does: with the question gate on it asks a person and reports
// true; with it off a blocked report blocks the run and reports true, and
// open questions ride along to the plan gate.
func (st *step) stopsToAsk(agentRun domain.AgentRun) bool {
	report := agentRun.Report
	if report == nil {
		return false
	}
	asked := questions(report)
	blocked := report.Status == "blocked"
	if !blocked && len(asked) == 0 {
		return false
	}
	if st.settings.QuestionGate(st.task) {
		if len(asked) == 0 {
			asked = []string{boundedText(nonEmpty(report.Summary, "The "+roleWord(agentRun.Role)+" could not finish and said no more."), 400)}
		}
		st.ask(agentRun, asked)
		return true
	}
	if blocked {
		st.block("the " + roleWord(agentRun.Role) + " could not go on: " + nonEmpty(report.Summary, "it gave no reason") + ".")
		return true
	}
	return false
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// lastReport returns the newest successful report of role in this run.
func (st *step) lastReport(role domain.Role) *domain.Report {
	agentRuns, err := st.tx.ListAgentRuns(st.ctx, domain.AgentRunFilter{RunID: st.run.ID, States: []domain.AgentRunState{domain.AgentSucceeded}, Limit: 1000})
	if err != nil {
		return nil
	}
	var latest *domain.Report
	for _, agentRun := range agentRuns {
		if agentRun.Role == role && agentRun.Report != nil {
			report := *agentRun.Report
			latest = &report
		}
	}
	return latest
}

// pass ends the current stage and enters the next one, or finishes the run
// after the last.
func (st *step) pass() {
	st.stage.State = domain.StagePassed
	st.stage.FinishedAt = &st.now
	st.event(domain.EventStageFinished, st.stage.ID, map[string]any{"kind": st.stage.Kind, "state": st.stage.State, "costUsd": st.stage.CostUSD, "turns": st.stage.Turns})
	next := domain.NextStage(st.stage.Kind, st.settings.RequirePRApproval)
	if next == "" {
		st.finishRun()
		return
	}
	stage := domain.NewStage(st.run.ID, next, st.stage.Ordinal+1)
	st.enter(&stage)
	st.newStages = append(st.newStages, stage)
}

// enter starts a stage: model stages queue their producer, approval stages
// ask a person, and non-model stages wait for the stage loop.
func (st *step) enter(stage *domain.Stage) {
	st.run.CurrentStageID, st.run.CurrentStageKind = stage.ID, stage.Kind
	switch stage.Kind {
	case domain.StagePlan, domain.StageImplement:
		stage.State, stage.Round, stage.StartedAt = domain.StageRunning, 1, &st.now
		role := domain.RolePlanner
		if stage.Kind == domain.StageImplement {
			role = domain.RoleCoder
		}
		agentRun := st.s.newAgentRun(st.run, *stage, role, st.settings, false)
		stage.Engine, stage.Model = agentRun.Engine, agentRun.Model
		st.agentRuns = append(st.agentRuns, agentRun)
	case domain.StagePlanApproval:
		stage.State, stage.StartedAt = domain.StageAwaitingApproval, &st.now
		summary := "The plan passed review."
		if report := st.lastReport(domain.RolePlanner); report != nil && report.Summary != "" {
			summary = report.Summary
		}
		st.request(domain.GatePlan, *stage, summary)
	case domain.StagePRApproval:
		stage.State, stage.StartedAt = domain.StageAwaitingApproval, &st.now
		st.request(domain.GateDraftPR, *stage, "The change passed its tests. Approve to push the branch and open a draft pull request.")
	default:
		stage.State = domain.StagePending
	}
	st.event(domain.EventStageStarted, stage.ID, map[string]any{"kind": stage.Kind, "state": stage.State, "round": stage.Round})
}

// finishRun marks the run passed and its task done.
func (st *step) finishRun() {
	st.run.State = domain.RunPassed
	st.run.FinishedAt = &st.now
	st.task.State = domain.TaskDone
	st.task.NeedsYou = false
	st.event(domain.EventRunFinished, st.stage.ID, map[string]any{"state": st.run.State, "costUsd": roundCents(st.run.Totals.CostUSD), "turns": st.run.Totals.Turns, "prUrl": st.run.PRURL})
}

// agentRunFinished moves a stage on after one of its agent runs ended.
func (s *Service) agentRunFinished(ctx context.Context, agentRun domain.AgentRun) {
	settings := s.settingsNow(ctx)
	err := s.atomic(ctx, func(tx domain.Repository, record func(domain.Event) error) error {
		st, err := s.loadStep(ctx, tx, record, settings, agentRun.RunID, agentRun.StageID)
		if err != nil {
			return err
		}
		if st.run.State != domain.RunRunning || st.stage.State != domain.StageRunning {
			return nil
		}
		st.addCost(agentRun)
		if agentRun.State != domain.AgentSucceeded {
			if strings.HasPrefix(agentRun.FailureReason, brief.ErrReportMissing.Error()) && !agentRun.Reminder {
				st.enqueue(agentRun.Role, true)
			} else {
				st.retryOrBlock(agentRun.Role, describeFailure(agentRun))
			}
			return st.save()
		}
		st.advance(agentRun)
		return st.save()
	})
	if err != nil && !errors.Is(err, domain.ErrStateMoved) {
		log.Printf("harness: move run %s on after agent run %s: %v", agentRun.RunID, agentRun.ID, err)
	}
	s.wakeWork()
}

func describeFailure(agentRun domain.AgentRun) string {
	reason := agentRun.FailureReason
	if reason == "" {
		reason = "failed"
	}
	return roleWord(agentRun.Role) + " " + reason + "."
}

// advance applies a successful agent run's report to its stage.
func (st *step) advance(agentRun domain.AgentRun) {
	report := agentRun.Report
	if producer(agentRun.Role) && st.stopsToAsk(agentRun) {
		return
	}
	switch agentRun.Role {
	case domain.RolePlanner:
		st.round(st.stage.Round).ProducerAgentRunID = agentRun.ID
		if report.Difficulty != "" {
			st.run.Difficulty = report.Difficulty
		}
		if report.PlanPath != "" {
			st.run.PlanPath = report.PlanPath
		}
		st.enqueue(domain.RolePlanReviewer, false)
	case domain.RolePlanReviewer, domain.RoleCodeReviewer:
		st.review(agentRun)
	default:
		st.advanceBuild(agentRun)
	}
}

// review records a reviewer's verdict: approve passes the stage; changes
// requested starts another round with the findings, or blocks the run after
// the last round.
func (st *step) review(agentRun domain.AgentRun) {
	report := agentRun.Report
	blocking, advisory := brief.Blocking(report.Findings)
	round := st.round(st.stage.Round)
	for _, finding := range report.Findings {
		finding.Round = st.stage.Round
		st.stage.Findings = append(st.stage.Findings, finding)
	}
	var thisRound []domain.Finding
	for _, finding := range st.stage.Findings {
		if finding.Round == st.stage.Round {
			thisRound = append(thisRound, finding)
		}
	}
	roundBlocking, roundAdvisory := brief.Blocking(thisRound)
	round.ReviewerAgentRunID, round.Verdict, round.Blocking, round.Advisory = agentRun.ID, report.Verdict, roundBlocking, roundAdvisory
	st.stage.Blocking, st.stage.Advisory = roundBlocking, roundAdvisory
	st.run.Totals.Blocking, st.run.Totals.Advisory = roundBlocking, roundAdvisory
	st.event(domain.EventStageRound, st.stage.ID, map[string]any{"kind": st.stage.Kind, "round": st.stage.Round, "verdict": report.Verdict, "blocking": blocking, "advisory": advisory})
	if report.Verdict == domain.VerdictApprove {
		st.reviewApproved(agentRun)
		return
	}
	if st.stage.Round >= st.stage.MaxRounds {
		st.block("the " + strings.ToLower(stageWord(st.stage.Kind)) + " still had changes requested after " + strconv.Itoa(st.stage.MaxRounds) + " rounds.")
		return
	}
	st.stage.Round++
	producerRole := domain.RolePlanner
	if st.stage.Kind == domain.StageImplement {
		producerRole = domain.RoleCoder
	}
	st.enqueue(producerRole, false)
}

func stageWord(kind domain.StageKind) string {
	switch kind {
	case domain.StagePlan:
		return "Plan"
	case domain.StagePlanApproval:
		return "Your approval"
	case domain.StageImplement:
		return "Build"
	case domain.StageQA:
		return "Test"
	case domain.StagePRApproval:
		return "Your approval of the pull request"
	case domain.StageDraftPR:
		return "Draft pull request"
	}
	return string(kind)
}

func roleWord(role domain.Role) string {
	switch role {
	case domain.RolePlanner:
		return "The planner"
	case domain.RolePlanReviewer:
		return "The plan checker"
	case domain.RoleCoder:
		return "The builder"
	case domain.RoleCodeReviewer:
		return "The code checker"
	case domain.RoleQAFixer:
		return "The tester"
	}
	return "The agent"
}
