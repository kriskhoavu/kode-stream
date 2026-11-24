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
	SourceLocal     SourceKind = "local"
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

// Every field of Source is untrusted.
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
	Reporter  *Person    `json:"reporter,omitempty"`
}

type Person struct {
	Name    string `json:"name"`
	Mention string `json:"mention,omitempty"`
}

// Every field of Comment is untrusted.
type Comment struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	Body      string    `json:"body"`
}

const (
	MaxTaskComments = 10
	MaxCommentBytes = 4 << 10
)

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
	WaitingFor       string    `json:"waitingFor,omitempty"`
	StartRequested   bool      `json:"startRequested"`
	AskQuestions     *bool     `json:"askQuestions,omitempty"`
	Note             string    `json:"note,omitempty"`
	RunID            string    `json:"runId,omitempty"`
	Attempts         int       `json:"attempts"`
	DependsOn        []string  `json:"dependsOn"`
	DependsOnKeys    []string  `json:"dependsOnKeys,omitempty"`
	EstimatedMinutes int       `json:"estimatedMinutes,omitempty"`
	Comments         []Comment `json:"comments,omitempty"`
	MissedPolls      int       `json:"missedPolls,omitempty"`
	Revision         int64     `json:"revision"`
	DiscoveredAt     time.Time `json:"discoveredAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type Candidate struct {
	WorkspaceID      string
	Source           Source
	Title            string
	Body             string
	DependsOnKeys    []string
	EstimatedMinutes int
	Comments         []Comment
}

func NewID(prefix string) string {
	var buf [6]byte
	_, _ = rand.Read(buf[:])
	return prefix + "-" + hex.EncodeToString(buf[:])
}
