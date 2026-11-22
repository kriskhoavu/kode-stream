// Package domain holds the harness's entities, state machines, policy,
// settings and the repository port that persists them.
package domain

import (
	"crypto/rand"
	"encoding/hex"
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

// NewID returns a prefixed random identifier such as "run-3f9a0c1d2e4b".
func NewID(prefix string) string {
	var buf [6]byte
	_, _ = rand.Read(buf[:])
	return prefix + "-" + hex.EncodeToString(buf[:])
}
