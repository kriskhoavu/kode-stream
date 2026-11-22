package harness

import (
	"context"
	"time"

	"github.com/google/uuid"

	"kode-stream/internal/harness/application/brief"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

// agentExecution is one claimed agent run on its way to its end.
type agentExecution struct {
	s        *Service
	ctx      context.Context
	agentRun domain.AgentRun
	settings domain.Settings
	lease    time.Duration
	run      domain.Run
	stage    domain.Stage
	task     domain.Task
}

// agentOutcome is what the harness learned after the CLI exited.
type agentOutcome struct {
	result     ports.Result
	report     domain.Report
	reportErr  error
	checkErr   error
	diffStat   string
	commit     string
	outOfScope []string
	gate       *domain.GateRun
}

// execute runs one claimed agent run to its end. It stores the CLI's usage,
// cost, session id and transcript path, and the bounded report; never the
// brief or the transcript.
func (s *Service) execute(agentRun domain.AgentRun, settings domain.Settings, pool domain.RoleKey) {
	ctx := s.workContext()
	defer func() {
		s.mu.Lock()
		s.busy[pool]--
		delete(s.handles, agentRun.ID)
		s.mu.Unlock()
		s.setProgress(agentRun.ID, "")
		s.wakeWork()
	}()
	execution := &agentExecution{s: s, ctx: ctx, agentRun: agentRun, settings: settings, lease: time.Duration(settings.Intervals.LeaseSeconds) * time.Second}
	if reason := execution.load(); reason != "" {
		execution.fail(reason)
		return
	}
	handle, engine, invocation, reason := execution.start()
	if reason != "" {
		execution.fail(reason)
		return
	}
	s.mu.Lock()
	s.handles[agentRun.ID] = agentProcess{runID: agentRun.RunID, pool: pool, handle: handle}
	s.mu.Unlock()
	execution.announce(engine, invocation)
	execution.watch(handle)
	result := handle.Wait()
	if ctx.Err() != nil {
		// The service is stopping: leave the row running for the next start
		// to interrupt and re-queue.
		return
	}
	execution.finish(execution.conclude(result))
}

func (e *agentExecution) fail(reason string) {
	e.s.finishAgentRun(e.ctx, e.agentRun.ID, func(current *domain.AgentRun) {
		current.State = domain.AgentFailed
		current.FailureReason = reason
	})
}

// load reads the run, stage and task the agent works for; a non-empty
// result is why it cannot.
func (e *agentExecution) load() string {
	var err error
	if e.run, err = e.s.repository.GetRun(e.ctx, e.agentRun.RunID); err != nil {
		return "run not found"
	}
	if e.stage, err = e.s.repository.GetStage(e.ctx, e.agentRun.StageID); err != nil {
		return "stage not found"
	}
	if e.task, err = e.s.repository.GetTask(e.ctx, e.run.TaskID); err != nil {
		return "task not found"
	}
	return ""
}

// start writes the brief and starts the CLI; a non-empty reason is why it did not start.
func (e *agentExecution) start() (ports.Handle, ports.Engine, ports.Invocation, string) {
	engine, ok := e.s.engines[e.agentRun.Engine]
	if !ok {
		return nil, nil, ports.Invocation{}, "engine " + e.agentRun.Engine + " is not available"
	}
	if err := brief.ClearReport(e.run.WorktreePath); err != nil {
		return nil, nil, ports.Invocation{}, "cannot prepare the worktree: " + err.Error()
	}
	policy := e.settings.For(e.run.WorkspaceID).ModelPolicy.Roles[e.agentRun.Role]
	invocation := ports.Invocation{
		Role: e.agentRun.Role, Brief: brief.Compose(e.s.briefInput(e.ctx, e.run, e.stage, e.task, e.agentRun, e.settings)), WorktreePath: e.run.WorktreePath,
		Model: e.agentRun.Model, MaxTurns: policy.MaxTurns, AllowedTools: policy.AllowedTools, DisallowedTools: e.settings.ModelPolicy.DisallowedTools,
		PermissionMode: policy.PermissionMode, MaxBudgetUSD: e.settings.ModelPolicy.BudgetFor(), SessionID: uuid.NewString(), Timeout: e.lease,
	}
	handle, err := engine.Start(e.ctx, invocation)
	if err != nil {
		return nil, nil, invocation, "the CLI did not start: " + boundedText(err.Error(), 400)
	}
	return handle, engine, invocation, ""
}

// announce records the session and tells the dashboard the agent started.
// The session id is the harness's own and the transcript path follows from
// it, so the dashboard can read the agent while it works.
func (e *agentExecution) announce(engine ports.Engine, invocation ports.Invocation) {
	transcriptRef := ""
	if locator, ok := engine.(ports.TranscriptLocator); ok {
		transcriptRef = locator.TranscriptRef(e.run.WorktreePath, invocation.SessionID)
	}
	e.s.recordSession(e.ctx, e.agentRun.ID, invocation.SessionID, transcriptRef)
	e.s.emit(e.ctx, domain.Event{Kind: domain.EventAgentStarted, TaskID: e.run.TaskID, RunID: e.run.ID, StageID: e.stage.ID, AgentRunID: e.agentRun.ID, Payload: payload(map[string]any{"role": e.agentRun.Role, "engine": e.agentRun.Engine, "model": e.agentRun.Model, "round": e.agentRun.Round, "attempt": e.agentRun.Attempt})})
}

// watch relays the CLI's progress and keeps the lease alive until its
// events end; a lost lease cancels the CLI.
func (e *agentExecution) watch(handle ports.Handle) {
	heartbeat := time.NewTicker(heartbeatEvery)
	defer heartbeat.Stop()
	var lastProgress time.Time
	events := handle.Events()
	for events != nil {
		select {
		case event, open := <-events:
			if !open {
				events = nil
				continue
			}
			if event.Kind == ports.EngineTurn && event.Summary != "" {
				e.s.setProgress(e.agentRun.ID, event.Summary)
				if time.Since(lastProgress) >= progressEvery {
					lastProgress = time.Now()
					e.s.publishTransient(domain.Event{Kind: domain.EventAgentProgress, TaskID: e.run.TaskID, RunID: e.run.ID, StageID: e.stage.ID, AgentRunID: e.agentRun.ID, Payload: payload(map[string]any{"progress": event.Summary, "turns": event.Turns, "costUsd": event.CostUSD})})
				}
			}
		case <-heartbeat.C:
			if !e.s.heartbeat(e.ctx, e.agentRun.ID, e.lease) {
				handle.Cancel()
			}
		}
	}
}

// conclude reads the report and, for work that produces or approves a
// change, checks, measures or commits it.
func (e *agentExecution) conclude(result ports.Result) agentOutcome {
	outcome := agentOutcome{result: result}
	outcome.report, outcome.reportErr = brief.ReadReport(e.run.WorktreePath, result.FinalText, e.agentRun.Role)
	succeeded := !result.IsError && outcome.reportErr == nil
	if succeeded && producer(e.agentRun.Role) {
		outcome.outOfScope, outcome.gate, outcome.checkErr = e.s.afterProducer(e.ctx, e.run, e.agentRun, e.settings, e.lease)
		if _, stat, err := e.s.worktrees.Diff(e.ctx, e.run.WorktreePath, e.run.BaseCommit); err == nil {
			outcome.diffStat = boundedText(stat, domain.MaxDiffStatBytes)
		}
	}
	if succeeded && e.agentRun.Role == domain.RoleCodeReviewer && outcome.report.Verdict == domain.VerdictApprove {
		outcome.commit, outcome.checkErr = e.s.commitRun(e.ctx, e.run, "")
	}
	return outcome
}

// finish stores the outcome on the agent run.
func (e *agentExecution) finish(outcome agentOutcome) {
	result := outcome.result
	e.s.finishAgentRun(e.ctx, e.agentRun.ID, func(current *domain.AgentRun) {
		if result.SessionID != "" {
			current.EngineSessionID = result.SessionID
		}
		if result.TranscriptRef != "" {
			current.TranscriptRef = result.TranscriptRef
		}
		current.Usage, current.CostUSD, current.Turns, current.DurationMs = result.Usage, result.CostUSD, result.Turns, result.DurationMs
		current.Usage.CostUSD = result.CostUSD
		exitCode := result.ExitCode
		current.ExitCode, current.StopReason, current.DiffStat = &exitCode, result.StopReason, outcome.diffStat
		current.OutOfScope, current.HarnessGate, current.Commit = outcome.outOfScope, outcome.gate, outcome.commit
		switch {
		case outcome.checkErr != nil:
			current.State = domain.AgentFailed
			current.FailureReason = "the harness could not check the work: " + boundedText(outcome.checkErr.Error(), 400)
		case result.IsError:
			current.State = domain.AgentFailed
			current.FailureReason = failureText(result)
		case outcome.reportErr != nil:
			current.State = domain.AgentFailed
			current.FailureReason = boundedText(outcome.reportErr.Error(), 600)
		default:
			current.State = domain.AgentSucceeded
			report := outcome.report
			current.Report = &report
		}
	})
}
