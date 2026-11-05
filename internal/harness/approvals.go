package harness

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

type Decision string

const (
	DecisionApprove Decision = "approve"
	DecisionReject  Decision = "reject"
)

type DecideInput struct {
	Decision       Decision
	Reason         string
	IdempotencyKey string
	// ExpectedRevision is the approval revision the decider saw; nil means
	// the current one.
	ExpectedRevision *int64
	RequestID        string
	Actor            domain.Actor
	Channel          domain.Channel
}

// AlreadyDecidedError carries the approval as it was decided.
type AlreadyDecidedError struct{ Approval domain.Approval }

func (e AlreadyDecidedError) Error() string { return domain.ErrAlreadyDecided.Error() }
func (e AlreadyDecidedError) Unwrap() error { return domain.ErrAlreadyDecided }

// Decide records a person's decision on a waiting approval and applies it.
// The same idempotency key again returns the recorded approval; a decision
// on an approval decided under another key is AlreadyDecidedError.
func (s *Service) Decide(ctx context.Context, approvalID string, input DecideInput) (domain.Approval, error) {
	if err := s.mutable(); err != nil {
		return domain.Approval{}, err
	}
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.Reason = boundedText(strings.TrimSpace(input.Reason), domain.MaxSummaryChars)
	if input.IdempotencyKey == "" || len(input.IdempotencyKey) > 200 {
		return domain.Approval{}, domain.Invalid("idempotencyKey is required and at most 200 characters")
	}
	if input.Decision != DecisionApprove && input.Decision != DecisionReject {
		return domain.Approval{}, domain.Invalid("decision must be approve or reject")
	}
	if input.Decision == DecisionReject && input.Reason == "" {
		return domain.Approval{}, domain.Invalid("a reason is required to ask for changes")
	}
	// A question gate's approval is the answer itself; it is checked once the
	// approval is loaded, below.
	if input.Channel == "" {
		input.Channel = domain.ChannelDashboard
	}
	if recorded, found, err := s.repository.FindApprovalByIdempotencyKey(ctx, input.IdempotencyKey); err != nil {
		return domain.Approval{}, err
	} else if found {
		if recorded.ID != approvalID {
			return domain.Approval{}, domain.Invalid("idempotencyKey was used for another decision")
		}
		return recorded, nil
	}
	settings := s.settingsNow(ctx)
	var decided domain.Approval
	err := s.atomic(ctx, func(tx domain.Repository, record func(domain.Event) error) error {
		approval, err := tx.GetApproval(ctx, approvalID)
		if err != nil {
			return err
		}
		if approval.State != domain.ApprovalRequested {
			return AlreadyDecidedError{Approval: approval}
		}
		if input.ExpectedRevision != nil && *input.ExpectedRevision != approval.Revision {
			return domain.ErrStateMoved
		}
		expect := domain.Expect{State: string(approval.State), Revision: approval.Revision}
		now := s.now()
		approval.State = domain.ApprovalApproved
		if input.Decision == DecisionReject {
			approval.State = domain.ApprovalRejected
		}
		actor := input.Actor
		approval.DecidedAt, approval.DecidedBy, approval.Channel, approval.Reason = &now, &actor, input.Channel, input.Reason
		approval.IdempotencyKey, approval.RequestID = input.IdempotencyKey, boundedText(input.RequestID, 200)
		if decided, err = tx.SaveApproval(ctx, approval, expect); err != nil {
			return err
		}
		if err := record(domain.Event{Kind: domain.EventApprovalDecided, TaskID: approval.TaskID, RunID: approval.RunID, StageID: approval.StageID, ApprovalID: approval.ID, Actor: actorOrNil(actor), Payload: payload(map[string]any{"gate": approval.Gate, "decision": input.Decision, "channel": input.Channel, "reason": input.Reason, "decidedBy": actor.DisplayName})}); err != nil {
			return err
		}
		return s.applyDecision(ctx, tx, record, settings, decided, input)
	})
	if err != nil {
		var already AlreadyDecidedError
		if errors.As(err, &already) {
			return already.Approval, err
		}
		return domain.Approval{}, err
	}
	if decided.Gate == domain.GatePlan && decided.State == domain.ApprovalApproved {
		s.commitApprovedPlan(ctx, decided.RunID)
	}
	s.wakeWork()
	return decided, nil
}

// commitApprovedPlan commits the approved plan on the run branch, so each
// builder's changes are measured against it.
func (s *Service) commitApprovedPlan(ctx context.Context, runID string) {
	run, err := s.repository.GetRun(ctx, runID)
	if err != nil {
		return
	}
	if commit, err := s.commitRun(ctx, run, "Plan "); err != nil {
		log.Printf("harness: commit the plan of run %s: %v", runID, err)
	} else if commit != "" {
		s.emit(ctx, domain.Event{Kind: domain.EventStageRound, TaskID: run.TaskID, RunID: run.ID, Payload: payload(map[string]any{"kind": domain.StagePlan, "commit": commit})})
	}
}

func (s *Service) applyDecision(ctx context.Context, tx domain.Repository, record func(domain.Event) error, settings domain.Settings, approval domain.Approval, input DecideInput) error {
	if approval.RunID == "" || approval.StageID == "" {
		return nil
	}
	st, err := s.loadStep(ctx, tx, record, settings, approval.RunID, approval.StageID)
	if err != nil {
		return err
	}
	approve := input.Decision == DecisionApprove
	switch approval.Gate {
	case domain.GateQuestion:
		if st.stage.State != domain.StageAwaitingApproval || st.run.State != domain.RunRunning {
			return domain.IllegalTransitionError{Entity: "stage", From: string(st.stage.State), To: "answered"}
		}
		if !approve {
			return s.cancelInTx(st, "Stopped at a question: "+input.Reason)
		}
		if input.Reason == "" {
			return domain.Invalid("an answer is required to continue")
		}
		st.task.NeedsYou = false
		st.stage.State = domain.StageRunning
		role := approval.Context.Role
		if role == "" {
			role = domain.RolePlanner
			if st.stage.Kind == domain.StageImplement {
				role = domain.RoleCoder
			}
		}
		st.stage.Answers = append(st.stage.Answers, domain.Answer{Role: role, Round: st.stage.Round, Questions: approval.Context.Questions, Answer: input.Reason, By: nonEmptyName(input.Actor)})
		st.enqueue(role, false)
		st.event(domain.EventStageRound, st.stage.ID, map[string]any{"kind": st.stage.Kind, "round": st.stage.Round, "answered": true, "role": role})
		return st.save()
	case domain.GatePlan, domain.GateDraftPR:
		if st.stage.State != domain.StageAwaitingApproval || st.run.State != domain.RunRunning {
			return domain.IllegalTransitionError{Entity: "stage", From: string(st.stage.State), To: "decided"}
		}
		st.task.NeedsYou = false
		if approve {
			st.pass()
			return st.save()
		}
		st.stage.Findings = append(st.stage.Findings, domain.Finding{Severity: "blocking", Title: "Changes asked by " + nonEmptyName(input.Actor), Detail: input.Reason})
		st.block("Changes were asked for: " + input.Reason)
		return st.save()
	case domain.GateResume:
		if st.run.State != domain.RunBlocked {
			return domain.IllegalTransitionError{Entity: "run", From: string(st.run.State), To: "resumed"}
		}
		if !approve {
			return s.cancelInTx(st, "Not resumed: "+input.Reason)
		}
		st.run.State, st.run.FailureReason = domain.RunRunning, ""
		st.task.State, st.task.NeedsYou = domain.TaskRunning, false
		resumed := domain.NewStage(st.run.ID, retryKind(st.stage.Kind), st.stage.Ordinal)
		for _, finding := range st.stage.Findings {
			if finding.Round == 0 || finding.Round == st.stage.Round {
				finding.Round = 0
				resumed.Findings = append(resumed.Findings, finding)
			}
		}
		if input.Reason != "" {
			resumed.Findings = append(resumed.Findings, domain.Finding{Severity: "advisory", Title: "Note from " + nonEmptyName(input.Actor), Detail: input.Reason})
		}
		st.enter(&resumed)
		st.newStages = append(st.newStages, resumed)
		st.event(domain.EventRunStarted, resumed.ID, map[string]any{"resumed": true, "stage": resumed.Kind})
		return st.save()
	}
	return nil
}

// retryKind is the stage a resume starts again: a rejected plan approval
// goes back to planning, a rejected PR approval back to testing.
func retryKind(kind domain.StageKind) domain.StageKind {
	switch kind {
	case domain.StagePlanApproval:
		return domain.StagePlan
	case domain.StagePRApproval:
		return domain.StageQA
	}
	return kind
}

func nonEmptyName(actor domain.Actor) string {
	if actor.DisplayName != "" {
		return actor.DisplayName
	}
	if actor.ActorID != "" {
		return actor.ActorID
	}
	return "a person"
}

// DecideRun decides the one approval a run is waiting on.
func (s *Service) DecideRun(ctx context.Context, runID string, input DecideInput) (domain.Approval, error) {
	if !s.Supported() {
		return domain.Approval{}, domain.ErrUnsupported
	}
	if recorded, found, err := s.repository.FindApprovalByIdempotencyKey(ctx, strings.TrimSpace(input.IdempotencyKey)); err == nil && found && recorded.RunID == runID {
		return recorded, nil
	}
	waiting, err := s.repository.ListApprovals(ctx, domain.ApprovalFilter{RunID: runID, States: []domain.ApprovalState{domain.ApprovalRequested}, Limit: 2})
	if err != nil {
		return domain.Approval{}, err
	}
	if len(waiting) == 0 {
		if _, err := s.repository.GetRun(ctx, runID); err != nil {
			return domain.Approval{}, err
		}
		return domain.Approval{}, domain.ErrNotFound
	}
	return s.Decide(ctx, waiting[0].ID, input)
}

// CancelRun stops a run: its CLI processes end, its stage and task are
// cancelled, and waiting approvals are superseded. The worktree is kept for
// the retention period.
func (s *Service) CancelRun(ctx context.Context, runID, reason string, expectedRevision *int64, actor domain.Actor) (domain.Run, error) {
	if err := s.mutable(); err != nil {
		return domain.Run{}, err
	}
	settings := s.settingsNow(ctx)
	var cancelled domain.Run
	err := s.atomic(ctx, func(tx domain.Repository, record func(domain.Event) error) error {
		run, err := tx.GetRun(ctx, runID)
		if err != nil {
			return err
		}
		if domain.RunTerminal(run.State) {
			return domain.IllegalTransitionError{Entity: "run", From: string(run.State), To: string(domain.RunCancelled)}
		}
		if expectedRevision != nil && *expectedRevision != run.Revision {
			return domain.ErrStateMoved
		}
		stageID := run.CurrentStageID
		if stageID == "" {
			return domain.ErrNotFound
		}
		st, err := s.loadStep(ctx, tx, record, settings, run.ID, stageID)
		if err != nil {
			return err
		}
		text := "Cancelled by " + nonEmptyName(actor)
		if reason = strings.TrimSpace(reason); reason != "" {
			text += ": " + boundedText(reason, 400)
		}
		if err := s.cancelInTx(st, text); err != nil {
			return err
		}
		cancelled = st.run
		cancelled.Revision++
		return nil
	})
	if err != nil {
		return domain.Run{}, err
	}
	s.killRun(runID)
	return cancelled, nil
}

// cancelInTx cancels the step's run, task, stage, queued and running agent
// runs and waiting approvals, and saves them.
func (s *Service) cancelInTx(st *step, reason string) error {
	if !domain.StageTerminal(st.stage.State) {
		st.stage.State = domain.StageCancelled
		st.stage.FinishedAt = &st.now
		st.stage.FailureReason = reason
	}
	st.run.State, st.run.FinishedAt, st.run.FailureReason = domain.RunCancelled, &st.now, reason
	st.task.State, st.task.NeedsYou, st.task.Note = domain.TaskCancelled, false, reason
	agentRuns, err := st.tx.ListAgentRuns(st.ctx, domain.AgentRunFilter{RunID: st.run.ID, States: []domain.AgentRunState{domain.AgentQueued, domain.AgentRunning, domain.AgentInterrupted}, Limit: 1000})
	if err != nil {
		return err
	}
	for _, agentRun := range agentRuns {
		expect := domain.Expect{State: string(agentRun.State), Revision: agentRun.Revision}
		agentRun.State, agentRun.FinishedAt, agentRun.FailureReason, agentRun.LeaseOwner, agentRun.LeaseUntil = domain.AgentCancelled, &st.now, reason, "", nil
		if _, err := st.tx.SaveAgentRun(st.ctx, agentRun, expect); err != nil {
			return err
		}
	}
	waiting, err := st.tx.ListApprovals(st.ctx, domain.ApprovalFilter{RunID: st.run.ID, States: []domain.ApprovalState{domain.ApprovalRequested}, Limit: 100})
	if err != nil {
		return err
	}
	for _, approval := range waiting {
		expect := domain.Expect{State: string(approval.State), Revision: approval.Revision}
		approval.State = domain.ApprovalSuperseded
		if _, err := st.tx.SaveApproval(st.ctx, approval, expect); err != nil {
			return err
		}
	}
	st.event(domain.EventStageFinished, st.stage.ID, map[string]any{"kind": st.stage.Kind, "state": st.stage.State})
	st.event(domain.EventRunFinished, st.stage.ID, map[string]any{"state": st.run.State, "reason": reason, "costUsd": roundCents(st.run.Totals.CostUSD)})
	return st.save()
}

// killRun stops the CLI processes of a run.
func (s *Service) killRun(runID string) {
	s.mu.Lock()
	var handles []ports.Handle
	for _, process := range s.handles {
		if process.runID == runID {
			handles = append(handles, process.handle)
		}
	}
	s.mu.Unlock()
	for _, handle := range handles {
		handle.Cancel()
	}
}

// --- reporting ---

// CostSummary sums recorded cost per day, run, role or model. It reports
// only; nothing acts on it.
func (s *Service) CostSummary(ctx context.Context, groupBy domain.CostGroup, from, to time.Time) (domain.CostSummary, error) {
	if groupBy == "" {
		groupBy = domain.CostByDay
	}
	if to.IsZero() {
		to = startOfDay(s.now()).Add(24 * time.Hour)
	}
	if from.IsZero() {
		from = to.Add(-30 * 24 * time.Hour)
	}
	if !from.Before(to) {
		return domain.CostSummary{}, domain.Invalid("from must be before to")
	}
	if !s.Supported() {
		return domain.CostSummary{GroupBy: groupBy, From: from, To: to, Rows: []domain.CostRow{}}, nil
	}
	summary, err := s.repository.CostSummary(ctx, domain.CostQuery{GroupBy: groupBy, From: from, To: to})
	if err != nil {
		return summary, err
	}
	if groupBy == domain.CostByRun {
		for i, row := range summary.Rows {
			if run, err := s.repository.GetRun(ctx, row.Key); err == nil {
				summary.Rows[i].Label = run.SourceKey + " · " + run.Title
			}
		}
	}
	summary.TotalUSD = roundCents(summary.TotalUSD)
	return summary, nil
}

type RunChanges struct {
	Branch     string           `json:"branch"`
	BaseCommit string           `json:"baseCommit"`
	Files      []ports.DiffFile `json:"files"`
	DiffStat   string           `json:"diffStat"`
	Available  bool             `json:"available"`
}

// Changes lists what a run changed in its worktree against its base.
func (s *Service) Changes(ctx context.Context, runID string) (RunChanges, error) {
	if !s.Supported() {
		return RunChanges{}, domain.ErrNotFound
	}
	run, err := s.repository.GetRun(ctx, runID)
	if err != nil {
		return RunChanges{}, err
	}
	changes := RunChanges{Branch: run.Branch, BaseCommit: run.BaseCommit, Files: []ports.DiffFile{}}
	if s.worktrees == nil || run.WorktreeRemoved || run.WorktreePath == "" {
		return changes, nil
	}
	files, stat, err := s.worktrees.Diff(ctx, run.WorktreePath, run.BaseCommit)
	if err != nil {
		return changes, nil
	}
	changes.Files, changes.DiffStat, changes.Available = files, boundedText(stat, domain.MaxDiffStatBytes), true
	return changes, nil
}
