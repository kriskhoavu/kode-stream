package ports

import (
	"context"
	"time"

	"kode-stream/internal/harness/domain"
)

// Engine is a spawn-and-parse adapter, not a provider abstraction: no routing, no auth, no guard.
type Engine interface {
	ID() string
	Detect(ctx context.Context) Detection
	Start(ctx context.Context, invocation Invocation) (Handle, error)
}

type Detection struct {
	Available   bool
	Version     string
	Path        string
	Unsupported []string
}

// Brief is never stored.
type Invocation struct {
	Role            domain.Role
	Brief           string
	WorktreePath    string
	Model           string
	MaxTurns        int
	AllowedTools    []string
	DisallowedTools []string
	PermissionMode  string
	MaxBudgetUSD    float64
	SessionID       string
	Timeout         time.Duration
	Env             map[string]string
}

type EngineEventKind string

const (
	EngineSession EngineEventKind = "session"
	EngineTurn    EngineEventKind = "turn"
	EngineText    EngineEventKind = "text"
)

type EngineEvent struct {
	Kind      EngineEventKind
	SessionID string
	Turns     int
	Tools     []string
	Summary   string
	CostUSD   float64
}

type Result struct {
	SessionID  string
	Turns      int
	DurationMs int64
	CostUSD    float64
	Usage      domain.Usage
	IsError    bool
	// StopReason is the CLI's own subtype, or cancelled or timeout.
	StopReason    string
	ExitCode      int
	FinalText     string
	Stderr        string
	TranscriptRef string
}

type TranscriptLocator interface {
	TranscriptRef(worktreePath, sessionID string) string
}

type TranscriptReader func(path string, limit int) ([]domain.TranscriptTurn, error)

type SessionCloser func(ctx context.Context, workspaceID, branch string) error

type Handle interface {
	Events() <-chan EngineEvent
	Cancel()
	Wait() Result
}
