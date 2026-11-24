package domain

import (
	"context"
	"time"
)

type Expect struct {
	State    string
	Revision int64
}

type TaskFilter struct {
	WorkspaceID string
	States      []TaskState
	Limit       int
}

type RunFilter struct {
	TaskID      string
	WorkspaceID string
	States      []RunState
	Limit       int
}

type AgentRunFilter struct {
	RunID   string
	StageID string
	States  []AgentRunState
	Limit   int
}

type ApprovalFilter struct {
	RunID   string
	TaskID  string
	StageID string
	States  []ApprovalState
	Limit   int
}

type CostGroup string

const (
	CostByDay   CostGroup = "day"
	CostByRun   CostGroup = "run"
	CostByRole  CostGroup = "role"
	CostByModel CostGroup = "model"
)

type CostQuery struct {
	GroupBy CostGroup
	From    time.Time
	To      time.Time
}

type CostRow struct {
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	CostUSD   float64 `json:"costUsd"`
	Turns     int     `json:"turns"`
	AgentRuns int     `json:"agentRuns"`
}

type CostSummary struct {
	GroupBy  CostGroup `json:"groupBy"`
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	TotalUSD float64   `json:"totalUsd"`
	Rows     []CostRow `json:"rows"`
}

type Claim struct {
	Owner      string
	LeaseUntil time.Time
	Now        time.Time
}

// Every Save is one conditional UPDATE on id, state and revision. Atomic's fn must use only the repository it is given.
type Repository interface {
	Atomic(ctx context.Context, fn func(Repository) error) error

	InsertTask(ctx context.Context, task Task) error
	GetTask(ctx context.Context, id string) (Task, error)
	FindTaskBySource(ctx context.Context, kind SourceKind, key string) (Task, bool, error)
	ListTasks(ctx context.Context, filter TaskFilter) ([]Task, error)
	SaveTask(ctx context.Context, task Task, expect Expect) (Task, error)
	DeleteTasks(ctx context.Context, filter TaskFilter) ([]Task, error)
	// Events stay: they are the log of what happened.
	DeleteTaskHistory(ctx context.Context, taskID string) error

	InsertRun(ctx context.Context, run Run) error
	GetRun(ctx context.Context, id string) (Run, error)
	ListRuns(ctx context.Context, filter RunFilter) ([]Run, error)
	SaveRun(ctx context.Context, run Run, expect Expect) (Run, error)

	InsertStage(ctx context.Context, stage Stage) error
	GetStage(ctx context.Context, id string) (Stage, error)
	ListStages(ctx context.Context, runID string) ([]Stage, error)
	SaveStage(ctx context.Context, stage Stage, expect Expect) (Stage, error)
	ClaimStage(ctx context.Context, kinds []StageKind, claim Claim) (Stage, bool, error)

	InsertAgentRun(ctx context.Context, agentRun AgentRun) error
	GetAgentRun(ctx context.Context, id string) (AgentRun, error)
	ListAgentRuns(ctx context.Context, filter AgentRunFilter) ([]AgentRun, error)
	SaveAgentRun(ctx context.Context, agentRun AgentRun, expect Expect) (AgentRun, error)
	ClaimAgentRun(ctx context.Context, roles []Role, claim Claim) (AgentRun, bool, error)
	// Expired agent runs go to interrupted, and expired stages back to pending.
	RecoverExpiredLeases(ctx context.Context, now time.Time) ([]AgentRun, []Stage, error)
	InterruptRunning(ctx context.Context, now time.Time) ([]AgentRun, error)

	InsertApproval(ctx context.Context, approval Approval) error
	GetApproval(ctx context.Context, id string) (Approval, error)
	FindApprovalByIdempotencyKey(ctx context.Context, key string) (Approval, bool, error)
	ListApprovals(ctx context.Context, filter ApprovalFilter) ([]Approval, error)
	SaveApproval(ctx context.Context, approval Approval, expect Expect) (Approval, error)

	AppendEvent(ctx context.Context, event Event) (Event, error)
	EventsAfter(ctx context.Context, after int64, limit int) ([]Event, error)
	LastEventSeq(ctx context.Context) (int64, error)

	EnqueueNotification(ctx context.Context, notification Notification) error
	DueNotifications(ctx context.Context, now time.Time, limit int) ([]Notification, error)
	SaveNotification(ctx context.Context, notification Notification) error

	LoadSettings(ctx context.Context) (Settings, error)
	SaveSettings(ctx context.Context, settings Settings, expectedRevision int64) (Settings, error)

	CostSummary(ctx context.Context, query CostQuery) (CostSummary, error)
}
