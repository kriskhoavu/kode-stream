package domain

import (
	"encoding/json"
	"time"
)

type ApprovalState string

const (
	ApprovalRequested  ApprovalState = "requested"
	ApprovalApproved   ApprovalState = "approved"
	ApprovalRejected   ApprovalState = "rejected"
	ApprovalExpired    ApprovalState = "expired"
	ApprovalSuperseded ApprovalState = "superseded"
)

type Channel string

const (
	ChannelDashboard Channel = "dashboard"
	ChannelWebex     Channel = "webex"
	ChannelAPI       Channel = "api"
	// ChannelSource is a decision the harness took on what the source said,
	// such as the reporter replying on a ticket.
	ChannelSource Channel = "source"
)

type Actor struct {
	ActorID     string `json:"actorId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
}

type ApprovalContext struct {
	Difficulty   string  `json:"difficulty,omitempty"`
	Subtasks     int     `json:"subtasks,omitempty"`
	CostSoFarUSD float64 `json:"costSoFarUsd"`
	Round        int     `json:"round,omitempty"`
	// Questions are what the agent asked (a question gate), or what the
	// planner left open for a person to settle (a plan gate).
	Questions []string `json:"questions,omitempty"`
	// Role is the agent that asked.
	Role Role `json:"role,omitempty"`
	// Reporter is who a clarify gate waits for, and Draft the comment a
	// person may post on the ticket to ask them.
	Reporter string `json:"reporter,omitempty"`
	Draft    string `json:"draft,omitempty"`
}

// MaxQuestions bounds how many questions one report may raise.
const MaxQuestions = 10

type Approval struct {
	ID             string          `json:"id"`
	RunID          string          `json:"runId,omitempty"`
	TaskID         string          `json:"taskId"`
	StageID        string          `json:"stageId,omitempty"`
	SourceKey      string          `json:"sourceKey"`
	Title          string          `json:"title"`
	Gate           GateKind        `json:"gate"`
	State          ApprovalState   `json:"state"`
	Summary        string          `json:"summary,omitempty"`
	Context        ApprovalContext `json:"context"`
	RequestedAt    time.Time       `json:"requestedAt"`
	DecidedAt      *time.Time      `json:"decidedAt,omitempty"`
	DecidedBy      *Actor          `json:"decidedBy,omitempty"`
	Channel        Channel         `json:"channel,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	IdempotencyKey string          `json:"-"`
	RequestID      string          `json:"-"`
	Revision       int64           `json:"revision"`
	ExpiresAt      *time.Time      `json:"expiresAt,omitempty"`
}

type EventKind string

const (
	EventTaskDiscovered     EventKind = "task.discovered"
	EventTaskUpdated        EventKind = "task.updated"
	EventTaskRanked         EventKind = "task.ranked"
	EventTaskPinned         EventKind = "task.pinned"
	EventTaskRemoved        EventKind = "task.removed"
	EventRunStarted         EventKind = "run.started"
	EventRunBlocked         EventKind = "run.blocked"
	EventRunFinished        EventKind = "run.finished"
	EventStageStarted       EventKind = "stage.started"
	EventStageRound         EventKind = "stage.round"
	EventStageFinished      EventKind = "stage.finished"
	EventAgentStarted       EventKind = "agent.started"
	EventAgentProgress      EventKind = "agent.progress"
	EventAgentFinished      EventKind = "agent.finished"
	EventApprovalRequested  EventKind = "approval.requested"
	EventApprovalDecided    EventKind = "approval.decided"
	EventRolePaused         EventKind = "role.paused"
	EventRoleResumed        EventKind = "role.resumed"
	EventSettingsUpdated    EventKind = "settings.updated"
	EventNotificationFailed EventKind = "notification.failed"
)

type Event struct {
	Seq        int64           `json:"seq"`
	Kind       EventKind       `json:"type"`
	At         time.Time       `json:"at"`
	TaskID     string          `json:"taskId,omitempty"`
	RunID      string          `json:"runId,omitempty"`
	StageID    string          `json:"stageId,omitempty"`
	AgentRunID string          `json:"agentRunId,omitempty"`
	ApprovalID string          `json:"approvalId,omitempty"`
	Actor      *Actor          `json:"actor,omitempty"`
	Payload    json.RawMessage `json:"payload"`
}

// NotificationState tracks one outbound webhook delivery.
type NotificationState string

const (
	NotificationPending   NotificationState = "pending"
	NotificationDelivered NotificationState = "delivered"
	NotificationFailed    NotificationState = "failed"
)

type Notification struct {
	ID        string            `json:"id"`
	EventSeq  int64             `json:"eventSeq"`
	Target    string            `json:"target"`
	State     NotificationState `json:"state"`
	Attempts  int               `json:"attempts"`
	NextAt    time.Time         `json:"nextAt"`
	LastError string            `json:"lastError,omitempty"`
}
