package ports

import (
	"context"
	"time"

	"kode-stream/internal/harness/domain"
)

// Engine runs one coding CLI headless. It is a spawn-and-parse adapter, not
// a provider abstraction: no routing, no auth, no guard of its own.
type Engine interface {
	ID() string
	// Detect reports whether the CLI is installed and what this adapter
	// cannot express for it; Settings shows the result.
	Detect(ctx context.Context) Detection
	Start(ctx context.Context, invocation Invocation) (Handle, error)
}

type Detection struct {
	Available   bool
	Version     string
	Path        string
	Unsupported []string
}

// Invocation is everything one agent run needs. Brief is never stored.
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
	// Env is the allow-listed environment beyond PATH and HOME.
	Env map[string]string
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
	// Summary is at most 200 characters of the CLI's latest text.
	Summary string
	CostUSD float64
}

type Result struct {
	SessionID  string
	Turns      int
	DurationMs int64
	CostUSD    float64
	Usage      domain.Usage
	IsError    bool
	// StopReason is the CLI's own subtype, such as success,
	// error_max_turns or error_max_budget_usd, or cancelled / timeout.
	StopReason string
	ExitCode   int
	FinalText  string
	Stderr     string
	// TranscriptRef is the path of the CLI's own transcript, never its
	// content.
	TranscriptRef string
}

// Handle is one running CLI process.
type Handle interface {
	Events() <-chan EngineEvent
	Cancel()
	Wait() Result
}
