// Package domain holds the harness's entities, state machines, policy,
// settings and the repository port that persists them.
package domain

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type SourceKind string

const (
	SourceJira      SourceKind = "jira"
	SourceGitHub    SourceKind = "github"
	SourceBitbucket SourceKind = "bitbucket"
	// SourceLocal reads tasks from Markdown files in a folder, for trying the
	// pipeline without an issue tracker; its hand-off pushes and opens no
	// pull request.
	SourceLocal SourceKind = "local"
)

type TaskState string

const (
	TaskDiscovered TaskState = "discovered"
	TaskRanked     TaskState = "ranked"
	TaskRunning    TaskState = "running"
	TaskBlocked    TaskState = "blocked"
	TaskDone       TaskState = "done"
	TaskDismissed  TaskState = "dismissed"
	TaskStale      TaskState = "stale"
	TaskCancelled  TaskState = "cancelled"
)

// Source is where a task came from. Every field is untrusted.
type Source struct {
	Kind      SourceKind `json:"kind"`
	Key       string     `json:"key"`
	URL       string     `json:"url"`
	Status    string     `json:"status,omitempty"`
	Labels    []string   `json:"labels"`
	Priority  string     `json:"priority,omitempty"`
	Assignee  string     `json:"assignee,omitempty"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
	Milestone string     `json:"milestone,omitempty"`
	Epic      string     `json:"epic,omitempty"`
	// Reporter is who opened the issue, where the source names one.
	Reporter *Person `json:"reporter,omitempty"`
}

// Person is someone at a task's source, as the source names them. Mention is
// the source's own markup that mentions them in a comment, where it has one.
type Person struct {
	Name    string `json:"name"`
	Mention string `json:"mention,omitempty"`
}

// Comment is one comment on a task's issue. Every field is untrusted.
type Comment struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	Body      string    `json:"body"`
}

// MaxTaskComments and MaxCommentBytes bound the comments kept on a task.
const (
	MaxTaskComments = 10
	MaxCommentBytes = 4 << 10
)

// MaxTaskBodyBytes bounds the untrusted task body kept on a task.
const MaxTaskBodyBytes = 16 << 10

type Task struct {
	ID               string    `json:"id"`
	WorkspaceID      string    `json:"workspaceId"`
	Source           Source    `json:"source"`
	Title            string    `json:"title"`
	Body             string    `json:"body,omitempty"`
	State            TaskState `json:"state"`
	Score            float64   `json:"score"`
	ScoreReasons     []string  `json:"scoreReasons"`
	PriorityOverride float64   `json:"priorityOverride"`
	Pinned           bool      `json:"pinned"`
	NeedsYou         bool      `json:"needsYou"`
	// WaitingFor names who the task's run waits for at a clarify gate,
	// rather than the person working on it.
	WaitingFor     string `json:"waitingFor,omitempty"`
	StartRequested bool   `json:"startRequested"`
	// AskQuestions overrides, for this task, whether an agent may stop and
	// ask a person; nil follows the workspace, then the settings.
	AskQuestions     *bool    `json:"askQuestions,omitempty"`
	Note             string   `json:"note,omitempty"`
	RunID            string   `json:"runId,omitempty"`
	Attempts         int      `json:"attempts"`
	DependsOn        []string `json:"dependsOn"`
	DependsOnKeys    []string `json:"dependsOnKeys,omitempty"`
	EstimatedMinutes int      `json:"estimatedMinutes,omitempty"`
	// Comments are the issue's latest comments, oldest first.
	Comments     []Comment `json:"comments,omitempty"`
	MissedPolls  int       `json:"missedPolls,omitempty"`
	Revision     int64     `json:"revision"`
	DiscoveredAt time.Time `json:"discoveredAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Candidate is what a scout found for one issue.
type Candidate struct {
	WorkspaceID      string
	Source           Source
	Title            string
	Body             string
	DependsOnKeys    []string
	EstimatedMinutes int
	Comments         []Comment
}

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

type AgentRunState string

const (
	AgentQueued      AgentRunState = "queued"
	AgentRunning     AgentRunState = "running"
	AgentSucceeded   AgentRunState = "succeeded"
	AgentFailed      AgentRunState = "failed"
	AgentCancelled   AgentRunState = "cancelled"
	AgentInterrupted AgentRunState = "interrupted"
)

type Usage struct {
	InputTokens      int64   `json:"inputTokens"`
	OutputTokens     int64   `json:"outputTokens"`
	CacheReadTokens  int64   `json:"cacheReadTokens"`
	CacheWriteTokens int64   `json:"cacheWriteTokens"`
	CostUSD          float64 `json:"costUsd"`
}

// MaxReportBytes and MaxDiffStatBytes bound what an agent run keeps.
const (
	MaxReportBytes   = 64 << 10
	MaxDiffStatBytes = 8 << 10
	MaxPayloadBytes  = 8 << 10
	MaxSummaryChars  = 600
	MaxDetailChars   = 800
	MaxProgressChars = 200
)

type AgentRun struct {
	ID              string        `json:"id"`
	RunID           string        `json:"runId"`
	StageID         string        `json:"stageId"`
	Role            Role          `json:"role"`
	Engine          string        `json:"engine"`
	Model           string        `json:"model"`
	State           AgentRunState `json:"state"`
	Round           int           `json:"round"`
	Attempt         int           `json:"attempt"`
	Reminder        bool          `json:"reminder,omitempty"`
	WorktreePath    string        `json:"worktreePath"`
	EngineSessionID string        `json:"engineSessionId,omitempty"`
	TranscriptRef   string        `json:"transcriptRef,omitempty"`
	Usage           Usage         `json:"usage"`
	CostUSD         float64       `json:"costUsd"`
	Turns           int           `json:"turns"`
	DurationMs      int64         `json:"durationMs"`
	ExitCode        *int          `json:"exitCode,omitempty"`
	StopReason      string        `json:"stopReason,omitempty"`
	FailureReason   string        `json:"failureReason,omitempty"`
	Report          *Report       `json:"report,omitempty"`
	DiffStat        string        `json:"diffStat,omitempty"`
	// OutOfScope lists paths the harness reverted after the agent changed
	// them outside its scope.
	OutOfScope []string `json:"outOfScope,omitempty"`
	// HarnessGate is the harness's own run of the verification commands.
	HarnessGate *GateRun `json:"harnessGate,omitempty"`
	// Commit is the harness's commit after this agent run, if any.
	Commit      string     `json:"commit,omitempty"`
	Progress    string     `json:"progress,omitempty"`
	LeaseOwner  string     `json:"-"`
	LeaseUntil  *time.Time `json:"-"`
	HeartbeatAt *time.Time `json:"heartbeatAt,omitempty"`
	QueuedAt    time.Time  `json:"queuedAt"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	Revision    int64      `json:"revision"`
}

// TranscriptTurn is one text turn of an agent's own transcript, read from the
// CLI's file when asked for and kept nowhere.
type TranscriptTurn struct {
	Role string    `json:"role"`
	Text string    `json:"text"`
	At   time.Time `json:"at,omitempty"`
}

// AgentRunSummary is the list form shown on a stage.
type AgentRunSummary struct {
	ID         string        `json:"id"`
	Role       Role          `json:"role"`
	Engine     string        `json:"engine"`
	Model      string        `json:"model"`
	State      AgentRunState `json:"state"`
	Round      int           `json:"round"`
	Attempt    int           `json:"attempt"`
	Turns      int           `json:"turns"`
	CostUSD    float64       `json:"costUsd"`
	DurationMs int64         `json:"durationMs"`
	StartedAt  *time.Time    `json:"startedAt,omitempty"`
	FinishedAt *time.Time    `json:"finishedAt,omitempty"`
	Progress   string        `json:"progress,omitempty"`
}

func (a AgentRun) Summary() AgentRunSummary {
	return AgentRunSummary{ID: a.ID, Role: a.Role, Engine: a.Engine, Model: a.Model, State: a.State, Round: a.Round, Attempt: a.Attempt, Turns: a.Turns, CostUSD: a.CostUSD, DurationMs: a.DurationMs, StartedAt: a.StartedAt, FinishedAt: a.FinishedAt, Progress: a.Progress}
}

// ReportSchema names the structured report version agents write.
const ReportSchema = "harness.report/v1"

// Answer is what a person replied when an agent stopped to ask.
type Answer struct {
	Role      Role     `json:"role"`
	Round     int      `json:"round"`
	Questions []string `json:"questions"`
	Answer    string   `json:"answer"`
	By        string   `json:"by,omitempty"`
	// FromSource marks a reply on the ticket itself, which the harness read
	// from the source, rather than an answer a person gave in Kode Stream.
	FromSource bool `json:"fromSource,omitempty"`
}

type Finding struct {
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Round    int    `json:"round,omitempty"`
}

type Gate struct {
	Command string `json:"command"`
	Passed  bool   `json:"passed"`
}

// MaxLogTailBytes bounds the verification output the harness keeps.
const MaxLogTailBytes = 8 << 10

// GateRun is one run of the verification commands by the harness itself.
type GateRun struct {
	Command    string `json:"command"`
	Passed     bool   `json:"passed"`
	ExitCode   int    `json:"exitCode"`
	DurationMs int64  `json:"durationMs"`
	// LogTail is the last 8 KB of the failing command's output. It is
	// untrusted and goes to a brief only inside task-data delimiters.
	LogTail string `json:"logTail,omitempty"`
	// Head is the commit a passing run verified, so Review can tell
	// whether anything changed after it.
	Head string `json:"head,omitempty"`
}

// Text is a report field an agent writes as words. A model sometimes writes
// it as a list of lines instead; Text takes both, joining a list one item a
// line, so the shape alone never loses a report.
type Text string

func (t *Text) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*t = Text(text)
		return nil
	}
	var lines []string
	if err := json.Unmarshal(data, &lines); err != nil {
		return fmt.Errorf("want text or a list of text: %w", err)
	}
	*t = Text(strings.Join(lines, "\n"))
	return nil
}

type Report struct {
	Schema       string    `json:"schema"`
	Status       string    `json:"status"`
	Summary      string    `json:"summary"`
	Difficulty   string    `json:"difficulty,omitempty"`
	Subtasks     []string  `json:"subtasks,omitempty"`
	PlanPath     string    `json:"planPath,omitempty"`
	Verdict      Verdict   `json:"verdict,omitempty"`
	Findings     []Finding `json:"findings,omitempty"`
	FilesChanged []string  `json:"filesChanged,omitempty"`
	Gate         *Gate     `json:"gate,omitempty"`
	PlanDrift    Text      `json:"planDrift,omitempty"`
	Open         []string  `json:"open,omitempty"`
}

type GateKind string

const (
	GateStart  GateKind = "start"
	GatePlan   GateKind = "plan"
	GateResume GateKind = "resume"
	// GateReview is the Review stage: approve hands the change off, reject
	// sends it back to Build, and replan back to Plan.
	GateReview GateKind = "review"
	// GateQuestion is an agent's own stop: it could not decide alone and
	// asks a person; approving sends the answer back into its round.
	GateQuestion GateKind = "question"
	// GateClarify waits for a ticket's reporter: the planner found the
	// ticket does not say enough to plan from. A new comment on the ticket
	// sends the planner out again; a person may also continue with what they
	// know, or dismiss the ticket.
	GateClarify GateKind = "clarify"
)

// ReportNeedsInfo is the status a planner reports when a ready ticket does
// not say enough to plan from.
const ReportNeedsInfo = "needs_info"

// MaxClarifyRounds bounds how often a planner waits for a ticket's reporter
// before it asks a person instead.
const MaxClarifyRounds = 3

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

// NewID returns a prefixed random identifier such as "run-3f9a0c1d2e4b".
func NewID(prefix string) string {
	var buf [6]byte
	_, _ = rand.Read(buf[:])
	return prefix + "-" + hex.EncodeToString(buf[:])
}
