package harness

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"kode-stream/internal/harness/application/brief"
	"kode-stream/internal/harness/application/scheduler"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

const (
	loopDispatcher = "dispatcher"
	loopStages     = "stages"
	heartbeatEvery = 30 * time.Second
	progressEvery  = time.Second
)

// workLoopNames are the loops woken whenever work may have appeared.
var workLoopNames = []string{loopDispatcher, string(domain.RoleKeyPlanner), string(domain.RoleKeyReviewer), string(domain.RoleKeyCoder), string(domain.RoleKeyQAFixer), loopStages}

// agentProcess is one running CLI and the run it belongs to.
type agentProcess struct {
	runID  string
	pool   domain.RoleKey
	handle ports.Handle
}

func (s *Service) workLoops() []scheduler.Loop {
	worker := s.interval(func(i domain.Intervals) int { return i.WorkerSeconds })
	loops := []scheduler.Loop{{Name: loopDispatcher, Interval: worker, Wake: s.workWakes[loopDispatcher], Run: s.dispatchCycle}}
	for _, pool := range []domain.RoleKey{domain.RoleKeyPlanner, domain.RoleKeyReviewer, domain.RoleKeyCoder, domain.RoleKeyQAFixer} {
		pool := pool
		loops = append(loops, scheduler.Loop{Name: string(pool), Interval: worker, Wake: s.workWakes[string(pool)], Run: func(ctx context.Context) error { return s.roleCycle(ctx, pool) }})
	}
	return append(loops, s.stageLoops(worker)...)
}

// cancelAgentProcesses stops every CLI this service started. Their rows stay
// running; the next start interrupts and re-queues them.
func (s *Service) cancelAgentProcesses() {
	s.mu.Lock()
	cancel := s.workCancel
	s.workCancel = nil
	s.workCtx = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// cancelPausedRole leaves a paused role's running agent runs to finish:
// pausing stops new claims, never work in flight.
func (s *Service) cancelPausedRole(domain.RoleKey) {}

func (s *Service) workContext() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workCtx == nil {
		s.workCtx, s.workCancel = context.WithCancel(context.Background())
	}
	return s.workCtx
}

// --- dispatcher ---

// dispatchCycle starts at most one run per workspace per cycle, taking each
// workspace's best startable task while it has fewer running runs than its
// maxConcurrentRuns. Workspaces take turns.
func (s *Service) dispatchCycle(ctx context.Context) error {
	settings := s.settingsNow(ctx)
	if !settings.Enabled || s.worktrees == nil || len(settings.Workspaces) == 0 {
		return nil
	}
	runs, err := s.repository.ListRuns(ctx, domain.RunFilter{States: []domain.RunState{domain.RunRunning}, Limit: 1000})
	if err != nil {
		return err
	}
	running := map[string]int{}
	for _, run := range runs {
		running[run.WorkspaceID]++
	}
	workspaces := append([]domain.WorkspaceSettings{}, settings.Workspaces...)
	sort.Slice(workspaces, func(i, j int) bool { return workspaces[i].WorkspaceID < workspaces[j].WorkspaceID })
	s.mu.Lock()
	start := s.dispatchCursor % len(workspaces)
	s.dispatchCursor++
	s.mu.Unlock()
	var errs []error
	for offset := range workspaces {
		workspace := workspaces[(start+offset)%len(workspaces)]
		if running[workspace.WorkspaceID] >= workspace.MaxConcurrentRuns {
			continue
		}
		task, found, err := s.nextStartable(ctx, settings, workspace.WorkspaceID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if found {
			if err := s.startRun(ctx, settings, task); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (s *Service) nextStartable(ctx context.Context, settings domain.Settings, workspaceID string) (domain.Task, bool, error) {
	tasks, err := s.repository.ListTasks(ctx, domain.TaskFilter{WorkspaceID: workspaceID, States: []domain.TaskState{domain.TaskRanked}, Limit: 500})
	if err != nil {
		return domain.Task{}, false, err
	}
	for _, task := range tasks {
		if !settings.AutoStart && !task.StartRequested {
			continue
		}
		ready := true
		for _, dependency := range task.DependsOn {
			blocker, err := s.repository.GetTask(ctx, dependency)
			if err == nil && blocker.State != domain.TaskDone && blocker.State != domain.TaskDismissed {
				ready = false
				break
			}
		}
		if ready {
			return task, true, nil
		}
	}
	return domain.Task{}, false, nil
}

var unsafeBranchRune = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func branchFor(key, runID string) string {
	slug := strings.Trim(unsafeBranchRune.ReplaceAllString(key, "-"), "-.")
	if len(slug) > 40 {
		slug = slug[:40]
	}
	if slug == "" {
		slug = "task"
	}
	return "harness/" + slug + "-" + strings.TrimPrefix(runID, "run-")[:6]
}

// startRun prepares the worktree, then creates the run, its plan stage and
// the first planner agent run in one transaction with the task's move to
// running.
func (s *Service) startRun(ctx context.Context, settings domain.Settings, task domain.Task) error {
	workspace, found, err := s.workspaces.Get(task.WorkspaceID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("workspace %s is not registered", task.WorkspaceID)
	}
	runID := domain.NewID("run")
	worktree, err := s.worktrees.Create(ctx, workspace, runID, branchFor(task.Source.Key, runID))
	if err != nil {
		s.noteTask(ctx, task, "Could not prepare a worktree: "+boundedText(err.Error(), 400))
		return err
	}
	now := s.now()
	run := domain.Run{ID: runID, TaskID: task.ID, WorkspaceID: task.WorkspaceID, SourceKey: task.Source.Key, Title: task.Title, Branch: worktree.Branch, WorktreePath: worktree.Path, BaseCommit: worktree.BaseCommit, State: domain.RunRunning, StartedAt: now}
	stage := domain.NewStage(runID, domain.StagePlan, 0)
	stage.State, stage.Round, stage.StartedAt = domain.StageRunning, 1, &now
	agentRun := s.newAgentRun(run, stage, domain.RolePlanner, settings, false)
	stage.Engine, stage.Model = agentRun.Engine, agentRun.Model
	run.CurrentStageID, run.CurrentStageKind = stage.ID, stage.Kind
	err = s.atomic(ctx, func(tx domain.Repository, record func(domain.Event) error) error {
		current, err := tx.GetTask(ctx, task.ID)
		if err != nil {
			return err
		}
		if current.State != domain.TaskRanked || current.Revision != task.Revision {
			return domain.ErrStateMoved
		}
		expect := domain.Expect{State: string(current.State), Revision: current.Revision}
		current.State, current.RunID, current.NeedsYou, current.Note, current.UpdatedAt = domain.TaskRunning, runID, false, "", now
		current.Attempts++
		if _, err := tx.SaveTask(ctx, current, expect); err != nil {
			return err
		}
		if err := tx.InsertRun(ctx, run); err != nil {
			return err
		}
		if err := tx.InsertStage(ctx, stage); err != nil {
			return err
		}
		if err := tx.InsertAgentRun(ctx, agentRun); err != nil {
			return err
		}
		if err := record(domain.Event{Kind: domain.EventRunStarted, TaskID: task.ID, RunID: runID, Payload: payload(map[string]any{"branch": run.Branch, "sourceKey": run.SourceKey, "title": run.Title})}); err != nil {
			return err
		}
		return record(domain.Event{Kind: domain.EventStageStarted, TaskID: task.ID, RunID: runID, StageID: stage.ID, Payload: payload(map[string]any{"kind": stage.Kind, "round": stage.Round})})
	})
	if err != nil {
		_ = s.worktrees.Remove(ctx, workspace, worktree.Path, worktree.Branch)
		if errors.Is(err, domain.ErrStateMoved) {
			return nil
		}
		return err
	}
	s.wakeWork()
	return nil
}

// noteTask records why a start failed and hands the task back to a person.
func (s *Service) noteTask(ctx context.Context, task domain.Task, note string) {
	expect := domain.Expect{State: string(task.State), Revision: task.Revision}
	task.Note, task.NeedsYou, task.StartRequested, task.UpdatedAt = note, true, false, s.now()
	if saved, err := s.repository.SaveTask(ctx, task, expect); err == nil {
		s.emit(ctx, domain.Event{Kind: domain.EventTaskUpdated, TaskID: saved.ID, Payload: payload(map[string]any{"state": saved.State, "note": note, "needsYou": true})})
	}
}

func (s *Service) newAgentRun(run domain.Run, stage domain.Stage, role domain.Role, settings domain.Settings, reminder bool) domain.AgentRun {
	policy := settings.ModelPolicy.Roles[role]
	return domain.AgentRun{ID: domain.NewID("agr"), RunID: run.ID, StageID: stage.ID, Role: role, Engine: policy.Engine, Model: settings.ModelPolicy.ModelFor(role, run.Difficulty), State: domain.AgentQueued, Round: stage.Round, Attempt: 1, Reminder: reminder, WorktreePath: run.WorktreePath, QueuedAt: s.now()}
}

// --- role loops ---

func (s *Service) roleCycle(ctx context.Context, pool domain.RoleKey) error {
	settings := s.settingsNow(ctx)
	if !active(settings, pool) {
		return nil
	}
	lease := time.Duration(settings.Intervals.LeaseSeconds) * time.Second
	for {
		s.mu.Lock()
		busy := s.busy[pool]
		s.mu.Unlock()
		if busy >= settings.Slots.For(pool) {
			return nil
		}
		now := s.now()
		agentRun, found, err := s.repository.ClaimAgentRun(ctx, domain.RolesOf(pool), domain.Claim{Owner: string(pool), LeaseUntil: now.Add(lease), Now: now})
		if err != nil || !found {
			return err
		}
		s.mu.Lock()
		s.busy[pool]++
		s.mu.Unlock()
		go s.execute(agentRun, settings, pool)
	}
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
	fail := func(reason string) {
		s.finishAgentRun(ctx, agentRun.ID, func(current *domain.AgentRun) {
			current.State = domain.AgentFailed
			current.FailureReason = reason
		})
	}
	run, err := s.repository.GetRun(ctx, agentRun.RunID)
	if err != nil {
		fail("run not found")
		return
	}
	stage, err := s.repository.GetStage(ctx, agentRun.StageID)
	if err != nil {
		fail("stage not found")
		return
	}
	task, err := s.repository.GetTask(ctx, run.TaskID)
	if err != nil {
		fail("task not found")
		return
	}
	engine, ok := s.engines[agentRun.Engine]
	if !ok {
		fail("engine " + agentRun.Engine + " is not available")
		return
	}
	if err := brief.ClearReport(run.WorktreePath); err != nil {
		fail("cannot prepare the worktree: " + err.Error())
		return
	}
	policy := settings.ModelPolicy.Roles[agentRun.Role]
	lease := time.Duration(settings.Intervals.LeaseSeconds) * time.Second
	invocation := ports.Invocation{
		Role: agentRun.Role, Brief: brief.Compose(s.briefInput(ctx, run, stage, task, agentRun, settings)), WorktreePath: run.WorktreePath,
		Model: agentRun.Model, MaxTurns: policy.MaxTurns, AllowedTools: policy.AllowedTools, DisallowedTools: settings.ModelPolicy.DisallowedTools,
		PermissionMode: policy.PermissionMode, MaxBudgetUSD: settings.ModelPolicy.BudgetFor(), SessionID: uuid.NewString(), Timeout: lease,
	}
	handle, err := engine.Start(ctx, invocation)
	if err != nil {
		fail("the CLI did not start: " + boundedText(err.Error(), 400))
		return
	}
	s.mu.Lock()
	s.handles[agentRun.ID] = agentProcess{runID: agentRun.RunID, pool: pool, handle: handle}
	s.mu.Unlock()
	// The session id is the harness's own and the transcript path follows
	// from it, so the dashboard can read the agent while it works.
	transcriptRef := ""
	if locator, ok := engine.(ports.TranscriptLocator); ok {
		transcriptRef = locator.TranscriptRef(run.WorktreePath, invocation.SessionID)
	}
	s.recordSession(ctx, agentRun.ID, invocation.SessionID, transcriptRef)
	s.emit(ctx, domain.Event{Kind: domain.EventAgentStarted, TaskID: run.TaskID, RunID: run.ID, StageID: stage.ID, AgentRunID: agentRun.ID, Payload: payload(map[string]any{"role": agentRun.Role, "engine": agentRun.Engine, "model": agentRun.Model, "round": agentRun.Round, "attempt": agentRun.Attempt})})

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
				s.setProgress(agentRun.ID, event.Summary)
				if time.Since(lastProgress) >= progressEvery {
					lastProgress = time.Now()
					s.publishTransient(domain.Event{Kind: domain.EventAgentProgress, TaskID: run.TaskID, RunID: run.ID, StageID: stage.ID, AgentRunID: agentRun.ID, Payload: payload(map[string]any{"progress": event.Summary, "turns": event.Turns, "costUsd": event.CostUSD})})
				}
			}
		case <-heartbeat.C:
			if !s.heartbeat(ctx, agentRun.ID, lease) {
				handle.Cancel()
			}
		}
	}
	result := handle.Wait()
	if ctx.Err() != nil {
		// The service is stopping: leave the row running for the next start
		// to interrupt and re-queue.
		return
	}
	report, reportErr := brief.ReadReport(run.WorktreePath, result.FinalText, agentRun.Role)
	succeeded := !result.IsError && reportErr == nil
	diffStat, commit := "", ""
	var outOfScope []string
	var gate *domain.GateRun
	var checkErr error
	if succeeded && producer(agentRun.Role) {
		outOfScope, gate, checkErr = s.afterProducer(ctx, run, agentRun, settings, lease)
		if _, stat, err := s.worktrees.Diff(ctx, run.WorktreePath, run.BaseCommit); err == nil {
			diffStat = boundedText(stat, domain.MaxDiffStatBytes)
		}
	}
	if succeeded && agentRun.Role == domain.RoleCodeReviewer && report.Verdict == domain.VerdictApprove {
		commit, checkErr = s.commitRun(ctx, run, "")
	}
	s.finishAgentRun(ctx, agentRun.ID, func(current *domain.AgentRun) {
		if result.SessionID != "" {
			current.EngineSessionID = result.SessionID
		}
		if result.TranscriptRef != "" {
			current.TranscriptRef = result.TranscriptRef
		}
		current.Usage, current.CostUSD, current.Turns, current.DurationMs = result.Usage, result.CostUSD, result.Turns, result.DurationMs
		current.Usage.CostUSD = result.CostUSD
		exitCode := result.ExitCode
		current.ExitCode, current.StopReason, current.DiffStat = &exitCode, result.StopReason, diffStat
		current.OutOfScope, current.HarnessGate, current.Commit = outOfScope, gate, commit
		switch {
		case checkErr != nil:
			current.State = domain.AgentFailed
			current.FailureReason = "the harness could not check the work: " + boundedText(checkErr.Error(), 400)
		case result.IsError:
			current.State = domain.AgentFailed
			current.FailureReason = failureText(result)
		case reportErr != nil:
			current.State = domain.AgentFailed
			current.FailureReason = boundedText(reportErr.Error(), 600)
		default:
			current.State = domain.AgentSucceeded
			current.Report = &report
		}
	})
}

func producer(role domain.Role) bool {
	return role == domain.RolePlanner || role == domain.RoleCoder || role == domain.RoleQAFixer
}

func failureText(result ports.Result) string {
	reason := result.StopReason
	switch reason {
	case "error_max_budget_usd":
		reason = "stopped at its spending limit"
	case "error_max_turns":
		reason = "ran out of turns"
	case "timeout":
		reason = "ran past its time limit"
	case "", "success":
		// The CLI ended without a stop reason of its own: a non-zero exit, or
		// a run cut short. Its last words say why ("Not logged in"), so they
		// are the reason; the exit code is the fallback.
		reason = "failed"
		if last := strings.TrimSpace(result.FinalText); last != "" {
			reason = "stopped: " + boundedText(last, 200)
		} else if result.ExitCode != 0 {
			reason = fmt.Sprintf("exited with code %d", result.ExitCode)
		}
	}
	if tail := strings.TrimSpace(result.Stderr); tail != "" {
		if len(tail) > 400 {
			tail = tail[len(tail)-400:]
		}
		reason += ": " + tail
	}
	return boundedText(reason, 600)
}

// heartbeat extends a running agent run's lease. It reports false when the
// row is no longer running, such as after a cancel.
// recordSession stores the session the CLI was started with and where its
// transcript will be, on the running row; a row that moved meanwhile is left.
func (s *Service) recordSession(ctx context.Context, agentRunID, sessionID, transcriptRef string) {
	current, err := s.repository.GetAgentRun(ctx, agentRunID)
	if err != nil || current.State != domain.AgentRunning {
		return
	}
	expect := domain.Expect{State: string(current.State), Revision: current.Revision}
	current.EngineSessionID, current.TranscriptRef = sessionID, transcriptRef
	if _, err := s.repository.SaveAgentRun(ctx, current, expect); err != nil && !errors.Is(err, domain.ErrStateMoved) {
		log.Printf("harness: record session of agent run %s: %v", agentRunID, err)
	}
}

func (s *Service) heartbeat(ctx context.Context, agentRunID string, lease time.Duration) bool {
	current, err := s.repository.GetAgentRun(ctx, agentRunID)
	if err != nil || current.State != domain.AgentRunning {
		return false
	}
	expect := domain.Expect{State: string(current.State), Revision: current.Revision}
	now := s.now()
	until := now.Add(lease)
	current.LeaseUntil, current.HeartbeatAt = &until, &now
	_, err = s.repository.SaveAgentRun(ctx, current, expect)
	return err == nil || !errors.Is(err, domain.ErrStateMoved)
}

// finishAgentRun records the end of a running agent run and moves its stage
// on. A row that left running meanwhile (a cancel) is left as it is.
func (s *Service) finishAgentRun(ctx context.Context, agentRunID string, apply func(*domain.AgentRun)) {
	for attempt := 0; attempt < 3; attempt++ {
		current, err := s.repository.GetAgentRun(ctx, agentRunID)
		if err != nil || current.State != domain.AgentRunning {
			return
		}
		expect := domain.Expect{State: string(current.State), Revision: current.Revision}
		apply(&current)
		now := s.now()
		current.FinishedAt, current.LeaseOwner, current.LeaseUntil, current.Progress = &now, "", nil, ""
		saved, err := s.repository.SaveAgentRun(ctx, current, expect)
		if errors.Is(err, domain.ErrStateMoved) {
			continue
		}
		if err != nil {
			log.Printf("harness: save agent run %s: %v", agentRunID, err)
			return
		}
		s.emit(ctx, domain.Event{Kind: domain.EventAgentFinished, RunID: saved.RunID, StageID: saved.StageID, AgentRunID: saved.ID, Payload: payload(map[string]any{"role": saved.Role, "state": saved.State, "costUsd": saved.CostUSD, "turns": saved.Turns, "stopReason": saved.StopReason, "failureReason": saved.FailureReason})})
		s.agentRunFinished(ctx, saved)
		return
	}
}

// briefInput gathers what a brief needs: the task, the run, the plan and
// the findings the producer must address.
func (s *Service) briefInput(ctx context.Context, run domain.Run, stage domain.Stage, task domain.Task, agentRun domain.AgentRun, settings domain.Settings) brief.Input {
	input := brief.Input{Role: agentRun.Role, Task: brief.TaskInfo{Key: task.Source.Key, Title: task.Title, URL: task.Source.URL, Body: task.Body}, Branch: run.Branch, BaseCommit: run.BaseCommit, Round: stage.Round, MaxRounds: stage.MaxRounds, Difficulty: run.Difficulty, PlanPath: run.PlanPath, Reminder: agentRun.Reminder}
	if workspace, found, err := s.workspaces.Get(run.WorkspaceID); err == nil && found {
		input.WorkspaceName, input.Baseline = workspace.Name, workspace.BaselineBranch
	}
	if workspaceSettings, ok := settings.Workspace(run.WorkspaceID); ok {
		input.GateCommands = workspaceSettings.VerificationCommands
	}
	if producer(agentRun.Role) {
		for _, finding := range stage.Findings {
			if finding.Round == 0 || finding.Round == stage.Round-1 {
				input.Findings = append(input.Findings, finding)
			}
		}
	}
	return s.decorateBrief(ctx, input, run, stage, agentRun)
}
