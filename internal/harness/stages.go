package harness

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/application/brief"
	"kode-stream/internal/harness/application/scheduler"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

// QAPasses is how many consecutive verification runs QA needs: two, so a
// flaky pass does not reach a pull request.
const QAPasses = 2

const janitorInterval = time.Hour

// draftRetryDelay spaces the draft PR stage's retries.
var draftRetryDelay = time.Minute

// stageLoops run the stages no model works: tests and the draft pull request,
// plus the worktree janitor.
func (s *Service) stageLoops(worker func() time.Duration) []scheduler.Loop {
	if worker == nil {
		worker = func() time.Duration { return time.Minute }
	}
	return []scheduler.Loop{
		{Name: loopStages, Interval: worker, Wake: s.workWakes[loopStages], Run: s.stageCycle},
		{Name: "janitor", Interval: func() time.Duration { return janitorInterval }, Run: s.janitorCycle},
	}
}

// stageCycle claims each pending QA and draft PR stage in turn.
func (s *Service) stageCycle(ctx context.Context) error {
	settings := s.settingsNow(ctx)
	if !settings.Enabled {
		return nil
	}
	lease := time.Duration(settings.Intervals.LeaseSeconds) * time.Second
	kinds := []domain.StageKind{domain.StageDraftPR}
	if !settings.Paused[domain.RoleKeyQAFixer] {
		kinds = append(kinds, domain.StageQA)
	}
	var errs []error
	for i := 0; i < 20; i++ {
		now := s.now()
		stage, found, err := s.repository.ClaimStage(ctx, kinds, domain.Claim{Owner: loopStages, LeaseUntil: now.Add(lease), Now: now})
		if err != nil || !found {
			return errors.Join(append(errs, err)...)
		}
		switch stage.Kind {
		case domain.StageQA:
			errs = append(errs, s.runQA(ctx, settings, stage, lease))
		case domain.StageDraftPR:
			errs = append(errs, s.runDraftPR(ctx, settings, stage))
		}
	}
	return errors.Join(errs...)
}

// qaCommands are the workspace's verification commands and its runtime's
// smoke verification, when it has one.
func (s *Service) qaCommands(settings domain.Settings, workspace models.WorkspaceConfig) []string {
	workspaceSettings, _ := settings.Workspace(workspace.ID)
	commands := append([]string{}, workspaceSettings.VerificationCommands...)
	if s.runtimeCmd != nil && workspace.Runtime != nil {
		if command := strings.TrimSpace(s.runtimeCmd(workspace.Runtime)); command != "" {
			commands = append(commands, command)
		}
	}
	return commands
}

// runQA runs the verification twice in the worktree. Both must pass; a
// failure goes to the tester with the log's end, for at most two fix rounds.
func (s *Service) runQA(ctx context.Context, settings domain.Settings, stage domain.Stage, lease time.Duration) error {
	run, err := s.repository.GetRun(ctx, stage.RunID)
	if err != nil {
		return err
	}
	workspace, _, err := s.workspaces.Get(run.WorkspaceID)
	if err != nil {
		return err
	}
	commands := s.qaCommands(settings, workspace)
	gate := &domain.GateRun{Passed: true}
	for pass := 0; pass < QAPasses && gate.Passed; pass++ {
		gate = runGate(ctx, run.WorktreePath, commands, lease)
	}
	commit := ""
	if gate.Passed && stage.Round > 0 {
		var err error
		if commit, err = s.commitRun(ctx, run, "Fix the tests of "); err != nil {
			gate = &domain.GateRun{Command: "git commit", Passed: false, ExitCode: -1, LogTail: err.Error()}
		} else if commit != "" {
			s.record(ctx, run, "harness.commit", models.AuditStatusSuccess, "Committed the tester's fixes", nil)
		}
	}
	err = s.atomic(ctx, func(tx domain.Repository, record func(domain.Event) error) error {
		st, err := s.loadStep(ctx, tx, record, settings, stage.RunID, stage.ID)
		if err != nil {
			return err
		}
		if st.stage.State != domain.StageRunning || st.stage.LeaseOwner != loopStages || st.run.State != domain.RunRunning {
			return nil
		}
		st.stage.LeaseOwner, st.stage.LeaseUntil, st.stage.LastGate = "", nil, gate
		st.stage.DurationMs += gate.DurationMs
		if gate.Passed {
			st.event(domain.EventStageRound, st.stage.ID, map[string]any{"kind": st.stage.Kind, "round": st.stage.Round, "gate": "passed", "passes": QAPasses, "commit": commit})
			st.pass()
			return st.save()
		}
		st.stage.Findings = append(st.stage.Findings, domain.Finding{Severity: "blocking", Title: "Tests failed: " + boundedText(gate.Command, 160), Detail: boundedText(tail(gate.LogTail, domain.MaxDetailChars), domain.MaxDetailChars), Round: st.stage.Round})
		st.stage.Blocking++
		if st.stage.Round >= st.stage.MaxRounds {
			st.block("the tests still failed after " + fmt.Sprint(st.stage.MaxRounds) + " fix rounds.")
			return st.save()
		}
		st.stage.Round++
		st.round(st.stage.Round)
		st.enqueue(domain.RoleQAFixer, false)
		st.event(domain.EventStageRound, st.stage.ID, map[string]any{"kind": st.stage.Kind, "round": st.stage.Round, "gate": "failed"})
		return st.save()
	})
	s.wakeWork()
	return err
}

// advanceFix sends a tester's fix back to QA, which runs the verification
// twice again.
func (st *step) advanceFix(agentRun domain.AgentRun) {
	st.round(st.stage.Round).ProducerAgentRunID = agentRun.ID
	if len(agentRun.OutOfScope) > 0 {
		st.stage.Findings = append(st.stage.Findings, domain.Finding{Severity: "advisory", Title: "Changes outside the scope were reverted", Detail: boundedText(strings.Join(agentRun.OutOfScope, ", "), domain.MaxDetailChars), Round: st.stage.Round})
		st.stage.Advisory++
	}
	st.stage.State, st.stage.LeaseOwner, st.stage.LeaseUntil = domain.StagePending, "", nil
}

// decorateFix gives the tester the failing log and the commands it must pass.
func (s *Service) decorateFix(_ context.Context, input brief.Input, stage domain.Stage, agentRun domain.AgentRun) brief.Input {
	if agentRun.Role == domain.RoleQAFixer && stage.LastGate != nil {
		input.LogTail = stage.LastGate.LogTail
	}
	return input
}

// runDraftPR pushes the run branch and opens a draft pull request, then
// finishes the run and removes its worktree.
func (s *Service) runDraftPR(ctx context.Context, settings domain.Settings, stage domain.Stage) error {
	run, err := s.repository.GetRun(ctx, stage.RunID)
	if err != nil {
		return err
	}
	workspace, _, err := s.workspaces.Get(run.WorkspaceID)
	if err != nil {
		return err
	}
	url, err := s.openPullRequest(ctx, settings, run, workspace)
	return s.finishDraftPR(ctx, settings, stage, run, workspace, url, err)
}

func (s *Service) openPullRequest(ctx context.Context, settings domain.Settings, run domain.Run, workspace models.WorkspaceConfig) (string, error) {
	workspaceSettings, _ := settings.Workspace(run.WorkspaceID)
	var adapter ports.PullRequests
	for _, candidate := range s.pulls {
		if candidate.Supports(workspace, workspaceSettings) {
			adapter = candidate
			break
		}
	}
	if adapter == nil {
		return "", errors.New("no pull request provider is configured for this workspace: set its GitHub repository or Bitbucket repository in Harness settings")
	}
	if s.worktrees == nil {
		return "", errors.New("worktrees are unavailable")
	}
	if err := s.worktrees.Push(ctx, run.WorktreePath, run.Branch); err != nil {
		s.record(ctx, run, "harness.push", models.AuditStatusFailed, "Push of "+run.Branch+" failed", err)
		return "", fmt.Errorf("push %s: %w", run.Branch, err)
	}
	s.record(ctx, run, "harness.push", models.AuditStatusSuccess, "Pushed "+run.Branch, nil)
	baseline := workspace.BaselineBranch
	if baseline == "" {
		baseline = "main"
	}
	url, err := adapter.CreateDraft(ctx, ports.PullRequest{Workspace: workspace, Settings: workspaceSettings, WorktreePath: run.WorktreePath, Branch: run.Branch, Base: baseline, Title: strings.SplitN(commitMessage(run, ""), "\n", 2)[0], Body: s.pullRequestBody(ctx, run)})
	if err != nil {
		s.record(ctx, run, "harness.pull_request", models.AuditStatusFailed, "Draft pull request failed", err)
		return "", err
	}
	s.record(ctx, run, "harness.pull_request", models.AuditStatusSuccess, "Opened draft pull request "+url, nil)
	return url, nil
}

func (s *Service) pullRequestBody(ctx context.Context, run domain.Run) string {
	var body strings.Builder
	fmt.Fprintf(&body, "Opened as a draft by the Kode Stream harness for %s.\n\n", run.SourceKey)
	if task, err := s.repository.GetTask(ctx, run.TaskID); err == nil && task.Source.URL != "" {
		fmt.Fprintf(&body, "Issue: %s\n", task.Source.URL)
	}
	fmt.Fprintf(&body, "Agent turns: %d · Cost: $%.2f\n\nA person reviews and merges this pull request; the harness never merges.\n", run.Totals.Turns, roundCents(run.Totals.CostUSD))
	return body.String()
}

func (s *Service) finishDraftPR(ctx context.Context, settings domain.Settings, stage domain.Stage, run domain.Run, workspace models.WorkspaceConfig, url string, openErr error) error {
	passed := false
	err := s.atomic(ctx, func(tx domain.Repository, record func(domain.Event) error) error {
		st, err := s.loadStep(ctx, tx, record, settings, stage.RunID, stage.ID)
		if err != nil {
			return err
		}
		if st.stage.State != domain.StageRunning || st.stage.LeaseOwner != loopStages || st.run.State != domain.RunRunning {
			return nil
		}
		st.stage.LeaseOwner, st.stage.LeaseUntil = "", nil
		if openErr == nil {
			st.run.PRURL = url
			st.pass()
			passed = true
			return st.save()
		}
		if st.stage.Attempt < st.stage.MaxAttempts {
			st.stage.Attempt++
			st.stage.State = domain.StagePending
			retryAt := st.now.Add(draftRetryDelay * time.Duration(st.stage.Attempt))
			st.stage.RetryAt = &retryAt
			st.stage.FailureReason = boundedText(openErr.Error(), 600)
			st.event(domain.EventStageRound, st.stage.ID, map[string]any{"kind": st.stage.Kind, "attempt": st.stage.Attempt, "retry": st.stage.FailureReason})
			return st.save()
		}
		st.block("the draft pull request could not be opened: " + openErr.Error())
		return st.save()
	})
	if err != nil {
		return err
	}
	if passed {
		s.removeWorktree(ctx, run, workspace, true)
	}
	return openErr
}

// removeWorktree deletes a run's worktree; deleteBranch also deletes the
// local branch, which is safe once it was pushed.
func (s *Service) removeWorktree(ctx context.Context, run domain.Run, workspace models.WorkspaceConfig, deleteBranch bool) {
	if s.worktrees == nil || run.WorktreePath == "" {
		return
	}
	branch := ""
	if deleteBranch {
		branch = run.Branch
	}
	if err := s.worktrees.Remove(ctx, workspace, run.WorktreePath, branch); err != nil {
		log.Printf("harness: remove the worktree of run %s: %v", run.ID, err)
	}
	for attempt := 0; attempt < 3; attempt++ {
		current, err := s.repository.GetRun(ctx, run.ID)
		if err != nil {
			return
		}
		expect := domain.Expect{State: string(current.State), Revision: current.Revision}
		current.WorktreeRemoved = true
		if _, err := s.repository.SaveRun(ctx, current, expect); !errors.Is(err, domain.ErrStateMoved) {
			return
		}
	}
}

// janitorCycle removes the worktrees of finished runs once their retention
// has passed. A blocked run keeps its worktree until it is resumed or
// cancelled.
func (s *Service) janitorCycle(ctx context.Context) error {
	settings := s.settingsNow(ctx)
	runs, err := s.repository.ListRuns(ctx, domain.RunFilter{States: []domain.RunState{domain.RunCancelled, domain.RunFailed, domain.RunPassed}, Limit: 1000})
	if err != nil {
		return err
	}
	cutoff := s.now().Add(-time.Duration(settings.RetentionDays) * 24 * time.Hour)
	for _, run := range runs {
		if run.WorktreeRemoved || run.WorktreePath == "" || run.FinishedAt == nil || run.FinishedAt.After(cutoff) {
			continue
		}
		workspace, found, err := s.workspaces.Get(run.WorkspaceID)
		if err != nil || !found {
			continue
		}
		s.removeWorktree(ctx, run, workspace, false)
	}
	return nil
}

// record writes an audit event for what the harness did to a repository.
func (s *Service) record(_ context.Context, run domain.Run, operation string, status models.AuditStatus, message string, err error) {
	if s.audit == nil {
		return
	}
	event := models.AuditEvent{ID: domain.NewID("aud"), Time: s.now(), OwnerUserID: "local", ActorUserID: "harness", WorkspaceID: run.WorkspaceID, Operation: operation, Status: status, Message: message, Paths: []string{run.Branch}}
	if err != nil {
		event.Error = boundedText(err.Error(), 600)
	}
	_, _ = s.audit.Append(event)
}
