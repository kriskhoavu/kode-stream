package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound       = errors.New("harness record not found")
	ErrStateMoved     = errors.New("harness record changed since it was loaded")
	ErrAlreadyDecided = errors.New("approval was already decided")
	ErrUnsupported    = errors.New("the harness needs SQLite or Postgres storage")
	ErrUnavailable    = errors.New("the harness runs in Local mode only")
	ErrDuplicate      = errors.New("harness record already exists")
)

type InvalidError struct{ Message string }

func (e InvalidError) Error() string { return e.Message }

func Invalid(format string, args ...any) error {
	return InvalidError{Message: fmt.Sprintf(format, args...)}
}

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
	// A stopped run is not finished work: the task may start again, be dismissed, or close at its source.
	TaskCancelled: {TaskRanked, TaskDismissed, TaskStale},
}

var runTransitions = map[RunState][]RunState{
	RunRunning: {RunPassed, RunBlocked, RunFailed, RunCancelled},
	RunBlocked: {RunRunning, RunCancelled, RunFailed},
}

var stageTransitions = map[StageState][]StageState{
	StagePending:          {StageRunning, StageAwaitingApproval, StageCancelled, StageFailed},
	StageRunning:          {StageRunning, StagePassed, StageAwaitingApproval, StageRetryWait, StageFailed, StageCancelled, StagePending},
	StageAwaitingApproval: {StagePassed, StageFailed, StageCancelled, StageRunning},
	StageRetryWait:        {StagePending, StageFailed, StageCancelled},
	StageFailed:           {StagePending},
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

func TaskTerminal(state TaskState) bool {
	return state == TaskDone || state == TaskDismissed
}

func TaskStartable(state TaskState) bool {
	return state == TaskRanked || state == TaskDiscovered || state == TaskCancelled
}

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

func StageSequence(requireReview bool) []StageKind {
	sequence := []StageKind{StagePlan, StagePlanApproval, StageImplement, StageQA}
	if requireReview {
		sequence = append(sequence, StageReview)
	}
	return append(sequence, StageDraftPR)
}

func NextStage(current StageKind, requireReview bool) StageKind {
	sequence := StageSequence(requireReview)
	for i, kind := range sequence {
		if kind == current && i+1 < len(sequence) {
			return sequence[i+1]
		}
	}
	return ""
}

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

func NewStage(runID string, kind StageKind, ordinal int) Stage {
	return Stage{ID: NewID("stg"), RunID: runID, Kind: kind, Ordinal: ordinal, State: StagePending, Round: 0, MaxRounds: MaxRoundsOf(kind), Attempt: 1, MaxAttempts: MaxStageAttempts, Rounds: []Round{}}
}
