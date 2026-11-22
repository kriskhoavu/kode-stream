package harness

import (
	"context"

	"kode-stream/internal/harness/domain"
)

func (s *Service) Tasks(ctx context.Context, filter domain.TaskFilter) ([]domain.Task, error) {
	if !s.Supported() {
		return []domain.Task{}, nil
	}
	tasks, err := s.repository.ListTasks(ctx, filter)
	if err != nil {
		return nil, err
	}
	return withoutBodies(tasks), nil
}

func withoutBodies(tasks []domain.Task) []domain.Task {
	for i := range tasks {
		tasks[i].Body = ""
	}
	return tasks
}

func (s *Service) Task(ctx context.Context, id string) (domain.Task, error) {
	if !s.Supported() {
		return domain.Task{}, domain.ErrNotFound
	}
	return s.repository.GetTask(ctx, id)
}

// Queue lists the tasks that can still be worked, stopped ones included, in
// the order the dispatcher would take them: pinned first, then by score.
func (s *Service) Queue(ctx context.Context, workspaceID string, limit int) ([]domain.Task, error) {
	return s.Tasks(ctx, domain.TaskFilter{WorkspaceID: workspaceID, States: []domain.TaskState{domain.TaskRanked, domain.TaskDiscovered, domain.TaskRunning, domain.TaskBlocked, domain.TaskCancelled}, Limit: limit})
}

type StageView struct {
	domain.Stage
	AgentRuns []domain.AgentRunSummary `json:"agentRuns"`
}

type RunView struct {
	domain.Run
	Stages []StageView `json:"stages,omitempty"`
}

func (s *Service) Runs(ctx context.Context, filter domain.RunFilter) ([]domain.Run, error) {
	if !s.Supported() {
		return []domain.Run{}, nil
	}
	return s.repository.ListRuns(ctx, filter)
}

func (s *Service) Run(ctx context.Context, id string) (RunView, error) {
	if !s.Supported() {
		return RunView{}, domain.ErrNotFound
	}
	run, err := s.repository.GetRun(ctx, id)
	if err != nil {
		return RunView{}, err
	}
	stages, err := s.repository.ListStages(ctx, id)
	if err != nil {
		return RunView{}, err
	}
	agentRuns, err := s.repository.ListAgentRuns(ctx, domain.AgentRunFilter{RunID: id, Limit: 1000})
	if err != nil {
		return RunView{}, err
	}
	byStage := map[string][]domain.AgentRunSummary{}
	for _, agentRun := range agentRuns {
		summary := agentRun.Summary()
		summary.Progress = s.progressOf(agentRun.ID)
		byStage[agentRun.StageID] = append(byStage[agentRun.StageID], summary)
	}
	view := RunView{Run: run, Stages: make([]StageView, 0, len(stages))}
	for _, stage := range stages {
		runs := byStage[stage.ID]
		if runs == nil {
			runs = []domain.AgentRunSummary{}
		}
		view.Stages = append(view.Stages, StageView{Stage: stage, AgentRuns: runs})
	}
	return view, nil
}

// AgentRunTranscript is the last limit text turns of an agent run's own
// transcript, read from the CLI's file now and kept nowhere. An agent run
// whose engine writes no readable transcript, or whose file does not exist
// yet, answers no turns.
type AgentRunTranscript struct {
	AgentRunID string                  `json:"agentRunId"`
	SessionID  string                  `json:"sessionId,omitempty"`
	Available  bool                    `json:"available"`
	Turns      []domain.TranscriptTurn `json:"turns"`
}

func (s *Service) AgentRunTranscript(ctx context.Context, id string, limit int) (AgentRunTranscript, error) {
	agentRun, err := s.AgentRun(ctx, id)
	if err != nil {
		return AgentRunTranscript{}, err
	}
	out := AgentRunTranscript{AgentRunID: agentRun.ID, SessionID: agentRun.EngineSessionID, Turns: []domain.TranscriptTurn{}}
	if s.transcripts == nil || agentRun.TranscriptRef == "" {
		return out, nil
	}
	turns, err := s.transcripts(agentRun.TranscriptRef, limit)
	if err != nil {
		return out, err
	}
	out.Available, out.Turns = true, turns
	if out.Turns == nil {
		out.Turns = []domain.TranscriptTurn{}
	}
	return out, nil
}

func (s *Service) AgentRun(ctx context.Context, id string) (domain.AgentRun, error) {
	if !s.Supported() {
		return domain.AgentRun{}, domain.ErrNotFound
	}
	agentRun, err := s.repository.GetAgentRun(ctx, id)
	if err != nil {
		return agentRun, err
	}
	if agentRun.State == domain.AgentRunning {
		agentRun.Progress = s.progressOf(id)
	}
	return agentRun, nil
}

func (s *Service) progressOf(agentRunID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress[agentRunID]
}

func (s *Service) setProgress(agentRunID, progress string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if progress == "" {
		delete(s.progress, agentRunID)
		return
	}
	s.progress[agentRunID] = progress
}

func (s *Service) Approvals(ctx context.Context, filter domain.ApprovalFilter) ([]domain.Approval, error) {
	if !s.Supported() {
		return []domain.Approval{}, nil
	}
	return s.repository.ListApprovals(ctx, filter)
}
