package harness

import (
	"context"
	"time"

	"kode-stream/internal/harness/domain"
)

type EngineStatus struct {
	ID          string   `json:"id"`
	Available   bool     `json:"available"`
	Version     string   `json:"version,omitempty"`
	Path        string   `json:"path,omitempty"`
	Unsupported []string `json:"unsupported"`
}

type RoleHealth struct {
	Role           domain.RoleKey `json:"role"`
	Paused         bool           `json:"paused"`
	Slots          int            `json:"slots"`
	Busy           int            `json:"busy"`
	LastActivityAt *time.Time     `json:"lastActivityAt,omitempty"`
	LastError      string         `json:"lastError,omitempty"`
}

type SourceHealth struct {
	WorkspaceID  string     `json:"workspaceId"`
	Kind         string     `json:"kind"`
	LastPolledAt *time.Time `json:"lastPolledAt,omitempty"`
	Found        int        `json:"found"`
	Added        int        `json:"added"`
	LastError    string     `json:"lastError,omitempty"`
}

type StatusCounts struct {
	WaitingApprovals int `json:"waitingApprovals"`
	Running          int `json:"running"`
	Queued           int `json:"queued"`
	DoneToday        int `json:"doneToday"`
	Blocked          int `json:"blocked"`
}

type Status struct {
	Enabled          bool           `json:"enabled"`
	StorageSupported bool           `json:"storageSupported"`
	Mode             string         `json:"mode"`
	Running          bool           `json:"running"`
	AutoStart        bool           `json:"autoStart"`
	AskQuestions     bool           `json:"askQuestions"`
	Engines          []EngineStatus `json:"engines"`
	Roles            []RoleHealth   `json:"roles"`
	Sources          []SourceHealth `json:"sources"`
	Counts           StatusCounts   `json:"counts"`
	CostTodayUSD     float64        `json:"costTodayUsd"`
	LastEventSeq     int64          `json:"lastEventSeq"`
	Hint             string         `json:"hint,omitempty"`
}

const StorageHint = "Switch Storage to SQLite in Settings and restart Kode Stream."

const CloudHint = "The harness runs coding CLIs on this machine, so it is available in Local mode only."

func (s *Service) Status(ctx context.Context) (Status, error) {
	status := Status{Mode: "local", Engines: []EngineStatus{}, Roles: []RoleHealth{}, Sources: []SourceHealth{}}
	if !s.config.LocalMode {
		status.Mode = "cloud"
		status.Hint = CloudHint
	}
	if !s.Supported() {
		status.Hint = StorageHint
		return status, nil
	}
	status.StorageSupported = true
	settings, err := s.repository.LoadSettings(ctx)
	if err != nil {
		return status, err
	}
	status.Enabled = settings.Enabled
	status.AutoStart = settings.AutoStart
	status.AskQuestions = settings.QuestionGate(domain.Task{})
	s.mu.Lock()
	status.Running = s.running
	s.mu.Unlock()
	busy := map[domain.RoleKey]int{}
	running, err := s.repository.ListAgentRuns(ctx, domain.AgentRunFilter{States: []domain.AgentRunState{domain.AgentRunning}, Limit: 1000})
	if err != nil {
		return status, err
	}
	for _, agentRun := range running {
		busy[domain.PoolOf(agentRun.Role)]++
	}
	for _, key := range domain.RoleKeys {
		health := RoleHealth{Role: key, Paused: settings.Paused[key], Slots: settings.Slots.For(key), Busy: busy[key]}
		if key == domain.RoleKeyScout || key == domain.RoleKeySorter {
			health.Slots = 1
		}
		status.Roles = append(status.Roles, s.decorateRole(health))
	}
	status.Sources = s.sourceHealth()
	status.Engines = s.engineStatus(ctx)
	waiting, err := s.repository.ListApprovals(ctx, domain.ApprovalFilter{States: []domain.ApprovalState{domain.ApprovalRequested}, Limit: 1000})
	if err != nil {
		return status, err
	}
	status.Counts.WaitingApprovals = len(waiting)
	tasks, err := s.repository.ListTasks(ctx, domain.TaskFilter{States: []domain.TaskState{domain.TaskRanked, domain.TaskDiscovered, domain.TaskBlocked, domain.TaskCancelled}, Limit: 1000})
	if err != nil {
		return status, err
	}
	for _, task := range tasks {
		if task.State == domain.TaskBlocked {
			status.Counts.Blocked++
		} else {
			status.Counts.Queued++
		}
	}
	runs, err := s.repository.ListRuns(ctx, domain.RunFilter{States: []domain.RunState{domain.RunRunning}, Limit: 1000})
	if err != nil {
		return status, err
	}
	status.Counts.Running = len(runs)
	dayStart := startOfDay(s.now())
	passed, err := s.repository.ListRuns(ctx, domain.RunFilter{States: []domain.RunState{domain.RunPassed}, Limit: 1000})
	if err != nil {
		return status, err
	}
	for _, run := range passed {
		if run.FinishedAt != nil && !run.FinishedAt.Before(dayStart) {
			status.Counts.DoneToday++
		}
	}
	cost, err := s.repository.CostSummary(ctx, domain.CostQuery{GroupBy: domain.CostByDay, From: dayStart, To: dayStart.Add(24 * time.Hour)})
	if err != nil {
		return status, err
	}
	status.CostTodayUSD = cost.TotalUSD
	status.LastEventSeq, err = s.repository.LastEventSeq(ctx)
	if !settings.Enabled && status.Hint == "" {
		status.Hint = "The harness is off. Turn it on in Harness settings."
	}
	return status, err
}

func startOfDay(now time.Time) time.Time {
	year, month, day := now.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func (s *Service) decorateRole(health RoleHealth) RoleHealth {
	s.mu.Lock()
	orchestrator := s.orchestrator
	s.mu.Unlock()
	if orchestrator == nil {
		return health
	}
	if loop, ok := orchestrator.Health(string(health.Role)); ok {
		health.LastActivityAt = loop.LastRunAt
		health.LastError = loop.LastError
	}
	return health
}

func (s *Service) sourceHealth() []SourceHealth {
	out := []SourceHealth{}
	if s.poller == nil {
		return out
	}
	for _, health := range s.poller.Health() {
		out = append(out, SourceHealth{WorkspaceID: health.WorkspaceID, Kind: string(health.Kind), LastPolledAt: health.LastPolledAt, Found: health.Found, Added: health.Added, LastError: health.LastError})
	}
	return out
}

// Detection runs each CLI's --version, so it runs at most once a minute.
func (s *Service) engineStatus(ctx context.Context) []EngineStatus {
	s.mu.Lock()
	if s.detections != nil && s.now().Sub(s.detectedAt) < time.Minute {
		cached := append([]EngineStatus{}, s.detections...)
		s.mu.Unlock()
		return cached
	}
	s.mu.Unlock()
	out := []EngineStatus{}
	for _, engine := range s.engineList {
		detection := engine.Detect(ctx)
		unsupported := detection.Unsupported
		if unsupported == nil {
			unsupported = []string{}
		}
		out = append(out, EngineStatus{ID: engine.ID(), Available: detection.Available, Version: detection.Version, Path: detection.Path, Unsupported: unsupported})
	}
	s.mu.Lock()
	s.detections = out
	s.detectedAt = s.now()
	s.mu.Unlock()
	return out
}
