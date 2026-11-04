package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound means the row does not exist.
	ErrNotFound = errors.New("harness record not found")
	// ErrStateMoved means a conditional transition matched no row: another
	// writer changed the state or the revision first.
	ErrStateMoved = errors.New("harness record changed since it was loaded")
	// ErrAlreadyDecided means an approval was decided under another key.
	ErrAlreadyDecided = errors.New("approval was already decided")
	// ErrUnsupported means the storage option cannot hold harness state.
	ErrUnsupported = errors.New("the harness needs SQLite or Postgres storage")
	// ErrUnavailable means the harness does not run in this mode.
	ErrUnavailable = errors.New("the harness runs in Local mode only")
	// ErrDuplicate means a uniqueness constraint refused the row.
	ErrDuplicate = errors.New("harness record already exists")
)

// InvalidError is a request the caller can correct.
type InvalidError struct{ Message string }

func (e InvalidError) Error() string { return e.Message }

func Invalid(format string, args ...any) error {
	return InvalidError{Message: fmt.Sprintf(format, args...)}
}

// IllegalTransitionError names a transition the state machine refuses.
type IllegalTransitionError struct {
	Entity string
	From   string
	To     string
}

func (e IllegalTransitionError) Error() string {
	return fmt.Sprintf("%s cannot move from %s to %s", e.Entity, e.From, e.To)
}

var taskTransitions = map[TaskState][]TaskState{
	TaskDiscovered: {TaskRanked, TaskDismissed, TaskStale},
	TaskRanked:     {TaskRanked, TaskRunning, TaskDismissed, TaskStale},
	TaskStale:      {TaskDiscovered, TaskDismissed},
	TaskRunning:    {TaskDone, TaskBlocked, TaskCancelled},
	TaskBlocked:    {TaskRunning, TaskCancelled, TaskDismissed},
}

var runTransitions = map[RunState][]RunState{
	RunRunning: {RunPassed, RunBlocked, RunFailed, RunCancelled},
	RunBlocked: {RunRunning, RunCancelled, RunFailed},
}

var stageTransitions = map[StageState][]StageState{
	StagePending:          {StageRunning, StageAwaitingApproval, StageCancelled, StageFailed},
	StageRunning:          {StageRunning, StagePassed, StageAwaitingApproval, StageRetryWait, StageFailed, StageCancelled, StagePending},
	StageAwaitingApproval: {StagePassed, StageFailed, StageCancelled},
	StageRetryWait:        {StagePending, StageFailed, StageCancelled},
	StageFailed:           {StagePending},
}

var agentTransitions = map[AgentRunState][]AgentRunState{
	AgentQueued:      {AgentRunning, AgentCancelled},
	AgentRunning:     {AgentSucceeded, AgentFailed, AgentCancelled, AgentInterrupted},
	AgentInterrupted: {AgentQueued, AgentFailed, AgentCancelled},
}

var approvalTransitions = map[ApprovalState][]ApprovalState{
	ApprovalRequested: {ApprovalApproved, ApprovalRejected, ApprovalExpired, ApprovalSuperseded},
}

func allowed[S ~string](table map[S][]S, from, to S) bool {
	for _, next := range table[from] {
		if next == to {
			return true
		}
	}
	return false
}

func CheckTask(from, to TaskState) error {
	if allowed(taskTransitions, from, to) {
		return nil
	}
	return IllegalTransitionError{Entity: "task", From: string(from), To: string(to)}
}

func CheckRun(from, to RunState) error {
	if allowed(runTransitions, from, to) {
		return nil
	}
	return IllegalTransitionError{Entity: "run", From: string(from), To: string(to)}
}

func CheckStage(from, to StageState) error {
	if allowed(stageTransitions, from, to) {
		return nil
	}
	return IllegalTransitionError{Entity: "stage", From: string(from), To: string(to)}
}

func CheckAgentRun(from, to AgentRunState) error {
	if allowed(agentTransitions, from, to) {
		return nil
	}
	return IllegalTransitionError{Entity: "agent run", From: string(from), To: string(to)}
}

func CheckApproval(from, to ApprovalState) error {
	if allowed(approvalTransitions, from, to) {
		return nil
	}
	return IllegalTransitionError{Entity: "approval", From: string(from), To: string(to)}
}

// TaskTerminal reports whether a task state never reopens.
func TaskTerminal(state TaskState) bool {
	return state == TaskDone || state == TaskDismissed || state == TaskCancelled
}

// TaskActive reports whether a task belongs to a run in flight.
func TaskActive(state TaskState) bool {
	return state == TaskRunning || state == TaskBlocked
}

func RunTerminal(state RunState) bool {
	return state == RunPassed || state == RunFailed || state == RunCancelled
}

func StageTerminal(state StageState) bool {
	return state == StagePassed || state == StageFailed || state == StageCancelled
}

func AgentTerminal(state AgentRunState) bool {
	return state == AgentSucceeded || state == AgentFailed || state == AgentCancelled
}

// StageSequence is the order stages are created in. pr_approval is skipped
// unless the settings require it.
func StageSequence(requirePRApproval bool) []StageKind {
	sequence := []StageKind{StagePlan, StagePlanApproval, StageImplement, StageQA}
	if requirePRApproval {
		sequence = append(sequence, StagePRApproval)
	}
	return append(sequence, StageDraftPR)
}

// NextStage returns the kind after current, or "" when current is the last.
func NextStage(current StageKind, requirePRApproval bool) StageKind {
	sequence := StageSequence(requirePRApproval)
	for i, kind := range sequence {
		if kind == current && i+1 < len(sequence) {
			return sequence[i+1]
		}
	}
	return ""
}

// MaxRoundsOf is the round bound of a stage kind.
func MaxRoundsOf(kind StageKind) int {
	switch kind {
	case StagePlan, StageImplement:
		return 3
	case StageQA:
		return 2
	default:
		return 1
	}
}

// NewStage builds a pending stage of a run.
func NewStage(runID string, kind StageKind, ordinal int) Stage {
	return Stage{ID: NewID("stg"), RunID: runID, Kind: kind, Ordinal: ordinal, State: StagePending, Round: 0, MaxRounds: MaxRoundsOf(kind), Attempt: 1, MaxAttempts: MaxStageAttempts, Rounds: []Round{}}
}
