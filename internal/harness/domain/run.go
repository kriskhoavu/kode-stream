package domain

import (
	"time"
)

type RunState string

const (
	RunRunning   RunState = "running"
	RunBlocked   RunState = "blocked"
	RunPassed    RunState = "passed"
	RunFailed    RunState = "failed"
	RunCancelled RunState = "cancelled"
)

type Totals struct {
	Turns      int     `json:"turns"`
	DurationMs int64   `json:"durationMs"`
	CostUSD    float64 `json:"costUsd"`
	Blocking   int     `json:"blocking"`
	Advisory   int     `json:"advisory"`
}

type Run struct {
	ID               string     `json:"id"`
	TaskID           string     `json:"taskId"`
	WorkspaceID      string     `json:"workspaceId"`
	SourceKey        string     `json:"sourceKey"`
	Title            string     `json:"title"`
	Branch           string     `json:"branch"`
	WorktreePath     string     `json:"worktreePath,omitempty"`
	BaseCommit       string     `json:"baseCommit,omitempty"`
	State            RunState   `json:"state"`
	CurrentStageID   string     `json:"currentStageId,omitempty"`
	CurrentStageKind StageKind  `json:"currentStageKind,omitempty"`
	Totals           Totals     `json:"totals"`
	PRURL            string     `json:"prUrl,omitempty"`
	Difficulty       string     `json:"difficulty,omitempty"`
	PlanPath         string     `json:"planPath,omitempty"`
	FailureReason    string     `json:"failureReason,omitempty"`
	WorktreeRemoved  bool       `json:"worktreeRemoved,omitempty"`
	Revision         int64      `json:"revision"`
	StartedAt        time.Time  `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt,omitempty"`
}

type StageKind string

const (
	StagePlan         StageKind = "plan"
	StagePlanApproval StageKind = "plan_approval"
	StageImplement    StageKind = "implement"
	StageQA           StageKind = "qa"
	StageReview       StageKind = "review"
	StageDraftPR      StageKind = "draft_pr"
)

type StageState string

const (
	StagePending          StageState = "pending"
	StageRunning          StageState = "running"
	StageAwaitingApproval StageState = "awaiting_approval"
	StageRetryWait        StageState = "retry_wait"
	StagePassed           StageState = "passed"
	StageFailed           StageState = "failed"
	StageCancelled        StageState = "cancelled"
)

type Verdict string

const (
	VerdictApprove          Verdict = "approve"
	VerdictChangesRequested Verdict = "changes_requested"
)

type Round struct {
	Number             int     `json:"number"`
	ProducerAgentRunID string  `json:"producerAgentRunId,omitempty"`
	ReviewerAgentRunID string  `json:"reviewerAgentRunId,omitempty"`
	Verdict            Verdict `json:"verdict,omitempty"`
	Blocking           int     `json:"blocking"`
	Advisory           int     `json:"advisory"`
}

const MaxStageAttempts = 3

type Stage struct {
	ID            string     `json:"id"`
	RunID         string     `json:"runId"`
	Kind          StageKind  `json:"kind"`
	Ordinal       int        `json:"ordinal"`
	State         StageState `json:"state"`
	Round         int        `json:"round"`
	MaxRounds     int        `json:"maxRounds"`
	Attempt       int        `json:"attempt"`
	MaxAttempts   int        `json:"maxAttempts"`
	Engine        string     `json:"engine,omitempty"`
	Model         string     `json:"model,omitempty"`
	Turns         int        `json:"turns"`
	DurationMs    int64      `json:"durationMs"`
	CostUSD       float64    `json:"costUsd"`
	Blocking      int        `json:"blocking"`
	Advisory      int        `json:"advisory"`
	FailureReason string     `json:"failureReason,omitempty"`
	LeaseOwner    string     `json:"-"`
	LeaseUntil    *time.Time `json:"-"`
	RetryAt       *time.Time `json:"retryAt,omitempty"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	Rounds        []Round    `json:"rounds"`
	Findings      []Finding  `json:"findings,omitempty"`
	Answers       []Answer   `json:"answers,omitempty"`
	LastGate      *GateRun   `json:"lastGate,omitempty"`
	Reviewed      bool       `json:"reviewed,omitempty"`
	Revision      int64      `json:"revision"`
}

type Role string

const (
	RolePlanner      Role = "planner"
	RolePlanReviewer Role = "plan_reviewer"
	RoleCoder        Role = "coder"
	RoleCodeReviewer Role = "code_reviewer"
	RoleQAFixer      Role = "qa_fixer"
)

var Roles = []Role{RolePlanner, RolePlanReviewer, RoleCoder, RoleCodeReviewer, RoleQAFixer}

type RoleKey string

const (
	RoleKeyScout    RoleKey = "scout"
	RoleKeySorter   RoleKey = "sorter"
	RoleKeyPlanner  RoleKey = "planner"
	RoleKeyReviewer RoleKey = "reviewer"
	RoleKeyCoder    RoleKey = "coder"
	RoleKeyQAFixer  RoleKey = "qa_fixer"
)

var RoleKeys = []RoleKey{RoleKeyScout, RoleKeySorter, RoleKeyPlanner, RoleKeyReviewer, RoleKeyCoder, RoleKeyQAFixer}

func ValidRoleKey(key RoleKey) bool {
	for _, candidate := range RoleKeys {
		if candidate == key {
			return true
		}
	}
	return false
}

func PoolOf(role Role) RoleKey {
	switch role {
	case RolePlanner:
		return RoleKeyPlanner
	case RolePlanReviewer, RoleCodeReviewer:
		return RoleKeyReviewer
	case RoleCoder:
		return RoleKeyCoder
	default:
		return RoleKeyQAFixer
	}
}

func RolesOf(pool RoleKey) []Role {
	switch pool {
	case RoleKeyPlanner:
		return []Role{RolePlanner}
	case RoleKeyReviewer:
		return []Role{RolePlanReviewer, RoleCodeReviewer}
	case RoleKeyCoder:
		return []Role{RoleCoder}
	case RoleKeyQAFixer:
		return []Role{RoleQAFixer}
	}
	return nil
}
