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
	// StageReview waits for a person to try the change, in a session of
	// their own in the run's worktree, and hand it off or send it back.
	StageReview  StageKind = "review"
	StageDraftPR StageKind = "draft_pr"
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

// MaxStageAttempts bounds how often a failed stage is retried.
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
	// Answers are a person's answers to the questions an agent of this stage
	// stopped to ask; every later brief of the stage carries them.
	Answers []Answer `json:"answers,omitempty"`
	// LastGate is the harness's latest verification run in a QA stage.
	LastGate *GateRun `json:"lastGate,omitempty"`
	// Reviewed marks a QA stage a person's hand-off started: it already
	// went through Review, so passing it goes on to the draft pull request.
	Reviewed bool  `json:"reviewed,omitempty"`
	Revision int64 `json:"revision"`
}

type Role string

const (
	RolePlanner      Role = "planner"
	RolePlanReviewer Role = "plan_reviewer"
	RoleCoder        Role = "coder"
	RoleCodeReviewer Role = "code_reviewer"
	RoleQAFixer      Role = "qa_fixer"
)

// Roles lists every agent role in pipeline order.
var Roles = []Role{RolePlanner, RolePlanReviewer, RoleCoder, RoleCodeReviewer, RoleQAFixer}

// RoleKey names a pausable worker: the two non-model loops and the four slot
// pools. Plan review and code review share the reviewer pool.
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

// PoolOf returns the slot pool an agent role draws from.
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

// RolesOf returns the agent roles that draw from a slot pool.
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
