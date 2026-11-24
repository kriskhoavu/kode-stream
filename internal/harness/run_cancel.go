package harness

import (
	"context"
	"strings"

	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

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

func (s *Service) cancelInTx(st *step, reason string) error {
	if !domain.StageTerminal(st.stage.State) {
		st.stage.State = domain.StageCancelled
		st.stage.FinishedAt = &st.now
		st.stage.FailureReason = reason
	}
	st.run.State, st.run.FinishedAt, st.run.FailureReason = domain.RunCancelled, &st.now, reason
	// The start request was spent on this run. Starting again asks anew.
	st.task.State, st.task.NeedsYou, st.task.Note, st.task.StartRequested, st.task.WaitingFor = domain.TaskCancelled, false, reason, false, ""
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
