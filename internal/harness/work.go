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

var workLoopNames = []string{loopDispatcher, string(domain.RoleKeyPlanner), string(domain.RoleKeyReviewer), string(domain.RoleKeyCoder), string(domain.RoleKeyQAFixer), loopStages}

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

// Their rows stay running. The next start interrupts and re-queues them.
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

func (s *Service) cancelPausedRole(domain.RoleKey) {}

func (s *Service) workContext() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workCtx == nil {
		s.workCtx, s.workCancel = context.WithCancel(context.Background())
	}
	return s.workCtx
}

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
		own := settings.For(workspace.WorkspaceID)
		if !own.Enabled || running[workspace.WorkspaceID] >= workspace.MaxConcurrentRuns {
			continue
		}
		task, found, err := s.nextStartable(ctx, own, workspace.WorkspaceID)
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
		if !settings.AutoStart && !task.StartRequested && !settings.ReadyCheck(task) {
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

func (s *Service) noteTask(ctx context.Context, task domain.Task, note string) {
	expect := domain.Expect{State: string(task.State), Revision: task.Revision}
	task.Note, task.NeedsYou, task.StartRequested, task.UpdatedAt = note, true, false, s.now()
	if saved, err := s.repository.SaveTask(ctx, task, expect); err == nil {
		s.emit(ctx, domain.Event{Kind: domain.EventTaskUpdated, TaskID: saved.ID, Payload: payload(map[string]any{"state": saved.State, "note": note, "needsYou": true})})
	}
}

func (s *Service) newAgentRun(run domain.Run, stage domain.Stage, role domain.Role, settings domain.Settings, reminder bool) domain.AgentRun {
	settings = settings.For(run.WorkspaceID)
	policy := settings.ModelPolicy.Roles[role]
	return domain.AgentRun{ID: domain.NewID("agr"), RunID: run.ID, StageID: stage.ID, Role: role, Engine: policy.Engine, Model: settings.ModelPolicy.ModelFor(role, run.Difficulty), State: domain.AgentQueued, Round: stage.Round, Attempt: 1, Reminder: reminder, WorktreePath: run.WorktreePath, QueuedAt: s.now()}
}

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
		// The CLI's last words say why it ended ("Not logged in"). The exit code is the fallback.
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

func (s *Service) briefInput(ctx context.Context, run domain.Run, stage domain.Stage, task domain.Task, agentRun domain.AgentRun, settings domain.Settings) brief.Input {
	input := brief.Input{Role: agentRun.Role, Task: brief.TaskInfo{Key: task.Source.Key, Title: task.Title, URL: task.Source.URL, Body: task.Body, Comments: task.Comments}, Branch: run.Branch, BaseCommit: run.BaseCommit, Round: stage.Round, MaxRounds: stage.MaxRounds, Difficulty: run.Difficulty, PlanPath: run.PlanPath, Reminder: agentRun.Reminder}
	if workspace, found, err := s.workspaces.Get(run.WorkspaceID); err == nil && found {
		input.WorkspaceName, input.Baseline = workspace.Name, workspace.BaselineBranch
	}
	if task.Source.Reporter != nil {
		input.Task.Reporter = task.Source.Reporter.Name
	}
	input.ReadyCheck = agentRun.Role == domain.RolePlanner && readinessOpen(settings, task, stage)
	if workspaceSettings, ok := settings.Workspace(run.WorkspaceID); ok {
		input.GateCommands = workspaceSettings.VerificationCommands
	}
	if producer(agentRun.Role) {
		for _, finding := range stage.Findings {
			if finding.Round == 0 || finding.Round == stage.Round-1 {
				input.Findings = append(input.Findings, finding)
			}
		}
		for _, answer := range stage.Answers {
			if answer.Role == agentRun.Role {
				input.Answers = append(input.Answers, answer)
			}
		}
	} else {
		for _, finding := range stage.Findings {
			if finding.Round == 0 {
				input.Requests = append(input.Requests, finding)
			}
		}
	}
	return s.decorateBrief(ctx, input, run, stage, agentRun)
}
