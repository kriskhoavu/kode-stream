package harness

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

type Decision string

const (
	DecisionApprove Decision = "approve"
	DecisionReject  Decision = "reject"
	// DecisionReplan answers Review only: it sends the change back to Plan.
	DecisionReplan Decision = "replan"
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

	// head is the run's commit once a Review decision committed the
	// reviewer's own changes; set by Decide, never by a caller.
	head string
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
	if input.Decision != DecisionApprove && input.Decision != DecisionReject && input.Decision != DecisionReplan {
		return domain.Approval{}, domain.Invalid("decision must be approve, reject or replan")
	}
	if input.Decision != DecisionApprove && input.Reason == "" {
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
	input.head = s.commitReview(ctx, approvalID)
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
		if input.Decision == DecisionReplan && approval.Gate != domain.GateReview {
			return domain.Invalid("replan answers Review only")
		}
		approval.State = domain.ApprovalApproved
		if input.Decision != DecisionApprove {
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
	case domain.GateClarify:
		return s.applyClarify(st, approval, input)
	case domain.GateReview:
		if st.stage.State != domain.StageAwaitingApproval || st.run.State != domain.RunRunning {
			return domain.IllegalTransitionError{Entity: "stage", From: string(st.stage.State), To: "decided"}
		}
		st.task.NeedsYou = false
		return s.applyReview(st, input)
	case domain.GatePlan:
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
// goes back to planning.
func retryKind(kind domain.StageKind) domain.StageKind {
	if kind == domain.StagePlanApproval {
		return domain.StagePlan
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
	// The start request was spent on this run; starting again asks anew.
	st.task.State, st.task.NeedsYou, st.task.Note, st.task.StartRequested = domain.TaskCancelled, false, reason, false
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
	// From says where the files were read: "worktree", "branch", or "".
	From string `json:"from"`
}

// RunFileDiff is the diff of one path a run changed.
type RunFileDiff struct {
	Path      string `json:"path"`
	Diff      string `json:"diff"`
	Truncated bool   `json:"truncated"`
	From      string `json:"from"`
}

// maxRunDiffBytes bounds one file's diff a person reads.
const maxRunDiffBytes = 512 << 10

// runSource is where a run's files are read: its workspace and its worktree
// while it has one.
func (s *Service) runSource(ctx context.Context, runID string) (domain.Run, models.WorkspaceConfig, string, bool, error) {
	if !s.Supported() {
		return domain.Run{}, models.WorkspaceConfig{}, "", false, domain.ErrNotFound
	}
	run, err := s.repository.GetRun(ctx, runID)
	if err != nil {
		return domain.Run{}, models.WorkspaceConfig{}, "", false, err
	}
	if s.worktrees == nil || s.workspaces == nil {
		return run, models.WorkspaceConfig{}, "", false, nil
	}
	workspace, found, err := s.workspaces.Get(run.WorkspaceID)
	if err != nil || !found {
		return run, workspace, "", false, err
	}
	worktree := run.WorktreePath
	if run.WorktreeRemoved {
		worktree = ""
	}
	return run, workspace, worktree, true, nil
}

// FileDiff returns the diff of one path the run changed: live from its
// worktree, else from its branch.
func (s *Service) FileDiff(ctx context.Context, runID, path string) (RunFileDiff, error) {
	run, workspace, worktree, ok, err := s.runSource(ctx, runID)
	if err != nil {
		return RunFileDiff{}, err
	}
	out := RunFileDiff{Path: path}
	if !ok {
		return out, nil
	}
	diff, truncated, from, err := s.worktrees.FileDiff(ctx, workspace, worktree, run.BaseCommit, run.Branch, path, maxRunDiffBytes)
	if errors.Is(err, ports.ErrNotChanged) {
		return out, domain.ErrNotFound
	}
	if err != nil {
		return out, err
	}
	out.Diff, out.Truncated, out.From = diff, truncated, from
	return out, nil
}

// Changes lists what a run changed in its worktree against its base.
func (s *Service) Changes(ctx context.Context, runID string) (RunChanges, error) {
	run, workspace, worktree, ok, err := s.runSource(ctx, runID)
	if err != nil {
		return RunChanges{}, err
	}
	changes := RunChanges{Branch: run.Branch, BaseCommit: run.BaseCommit, Files: []ports.DiffFile{}}
	if !ok {
		return changes, nil
	}
	files, from, err := s.worktrees.ChangedFiles(ctx, workspace, worktree, run.BaseCommit, run.Branch)
	if err != nil || from == "" {
		return changes, nil
	}
	changes.Files, changes.From, changes.Available = files, from, true
	return changes, nil
}

// RunPlan is the plan a run wrote, readable before it is merged.
type RunPlan struct {
	Branch string          `json:"branch"`
	From   string          `json:"from"`
	Files  []ports.RunFile `json:"files"`
}

// maxPlanFileBytes bounds one plan file a person reads.
const maxPlanFileBytes = 256 << 10

// Plan reads the files a run wrote under the plan folder: live from its
// worktree while it has one, else from its branch. A run whose worktree and
// branch are both gone has no plan to show here; once merged, it is on the
// board.
func (s *Service) Plan(ctx context.Context, runID string) (RunPlan, error) {
	run, workspace, worktree, ok, err := s.runSource(ctx, runID)
	if err != nil {
		return RunPlan{}, err
	}
	plan := RunPlan{Branch: run.Branch, Files: []ports.RunFile{}}
	if !ok {
		return plan, nil
	}
	files, from, err := s.worktrees.ReadRunFiles(ctx, workspace, worktree, run.BaseCommit, run.Branch, planScope[0], maxPlanFileBytes)
	if err != nil {
		return plan, nil
	}
	plan.Files, plan.From = files, from
	return plan, nil
}

// commitReview commits what a person changed in the run's worktree during
// Review, before any decision on it, so a handed-off change carries it and
// a builder sent back starts from it. It returns the run's commit after.
func (s *Service) commitReview(ctx context.Context, approvalID string) string {
	approval, err := s.repository.GetApproval(ctx, approvalID)
	if err != nil || approval.Gate != domain.GateReview || approval.State != domain.ApprovalRequested {
		return ""
	}
	run, err := s.repository.GetRun(ctx, approval.RunID)
	if err != nil {
		return ""
	}
	if commit, err := s.commitRun(ctx, run, "Apply the review of "); err != nil {
		log.Printf("harness: commit the review of run %s: %v", run.ID, err)
	} else if commit != "" {
		s.record(ctx, run, "harness.commit", models.AuditStatusSuccess, "Committed the changes made in Review", nil)
	}
	return s.head(ctx, run)
}

// head is the commit a run's worktree is at, or "" when it is unknown.
func (s *Service) head(ctx context.Context, run domain.Run) string {
	if s.worktrees == nil || run.WorktreePath == "" {
		return ""
	}
	head, err := s.worktrees.Head(ctx, run.WorktreePath)
	if err != nil {
		return ""
	}
	return head
}

// applyReview applies a person's Review decision. A hand-off goes on to the
// draft pull request, through the tests again when the change moved after
// they passed. Reject sends the change back to Build, replan back to Plan,
// each with the reviewer's words as a blocking finding; both come back to
// Review once their tests pass.
func (s *Service) applyReview(st *step, input DecideInput) error {
	if input.Decision == DecisionApprove {
		tested := ""
		if stages, err := st.tx.ListStages(st.ctx, st.run.ID); err == nil {
			for _, stage := range stages {
				if stage.Kind == domain.StageQA && stage.LastGate != nil && stage.LastGate.Passed {
					tested = stage.LastGate.Head
				}
			}
		}
		if input.head == tested {
			st.pass()
			return st.save()
		}
		st.close()
		retest := domain.NewStage(st.run.ID, domain.StageQA, st.stage.Ordinal+1)
		retest.Reviewed = true
		st.enter(&retest)
		st.newStages = append(st.newStages, retest)
		return st.save()
	}
	kind, where := domain.StageImplement, "Build"
	if input.Decision == DecisionReplan {
		kind, where = domain.StagePlan, "Plan"
	}
	st.stage.Findings = append(st.stage.Findings, domain.Finding{Severity: "advisory", Title: "Sent back to " + where + " by " + nonEmptyName(input.Actor), Detail: input.Reason})
	st.close()
	again := domain.NewStage(st.run.ID, kind, st.stage.Ordinal+1)
	again.Findings = []domain.Finding{{Severity: "blocking", Title: "Asked in Review by " + nonEmptyName(input.Actor), Detail: input.Reason}}
	st.enter(&again)
	st.newStages = append(st.newStages, again)
	st.event(domain.EventStageRound, st.stage.ID, map[string]any{"kind": st.stage.Kind, "sentBackTo": kind})
	return st.save()
}

// ReviewTarget is where a person's Review session runs: the run's own
// worktree, on its branch, with a brief that says what they are looking at.
type ReviewTarget struct {
	WorkspaceID string
	Dir         string
	Branch      string
	Prompt      string
}

// ReviewSession names the worktree of a run waiting at Review. A run at any
// other stage, or without a worktree, has none.
func (s *Service) ReviewSession(ctx context.Context, runID string) (ReviewTarget, error) {
	if !s.Supported() {
		return ReviewTarget{}, domain.ErrNotFound
	}
	run, err := s.repository.GetRun(ctx, runID)
	if err != nil {
		return ReviewTarget{}, err
	}
	if run.State != domain.RunRunning || run.CurrentStageKind != domain.StageReview {
		return ReviewTarget{}, domain.IllegalTransitionError{Entity: "run", From: string(run.CurrentStageKind), To: "review session"}
	}
	if run.WorktreePath == "" || run.WorktreeRemoved {
		return ReviewTarget{}, domain.Invalid("this run has no worktree left to open")
	}
	task, err := s.repository.GetTask(ctx, run.TaskID)
	if err != nil {
		return ReviewTarget{}, err
	}
	var prompt strings.Builder
	prompt.WriteString("You are helping a person review a change the harness made for " + task.Source.Key + ": " + task.Title + ".\n")
	prompt.WriteString("This folder is the run's own worktree, on branch " + run.Branch + ". The change passed its tests.\n")
	if run.PlanPath != "" {
		prompt.WriteString("The approved plan is " + run.PlanPath + ".\n")
	}
	prompt.WriteString("Start by summarising what changed (git log and git diff against the base). Then help the person try it and make the changes they ask for. Do not push. The harness commits what is left when the person hands the change off.")
	return ReviewTarget{WorkspaceID: run.WorkspaceID, Dir: run.WorktreePath, Branch: run.Branch, Prompt: prompt.String()}, nil
}
