package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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
