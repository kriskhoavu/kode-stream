// Package notify writes the outbox row with the event, so delivery is at least once and the receiver dedupes on the event id.
package notify

import (
	"fmt"
	"time"

	"kode-stream/internal/harness/domain"
)

const (
	MaxAttempts = 5
	firstDelay  = 5 * time.Second
	maxDelay    = 10 * time.Minute
	Target      = "bot-webhook"
)

var Notifiable = map[domain.EventKind]bool{
	domain.EventApprovalRequested: true,
	domain.EventStageFinished:     true,
	domain.EventRunBlocked:        true,
	domain.EventRunFinished:       true,
}

func Backoff(attempts int) time.Duration {
	delay := firstDelay
	for i := 1; i < attempts; i++ {
		delay *= 2
		if delay >= maxDelay {
			return maxDelay
		}
	}
	return delay
}

func EventID(seq int64) string { return fmt.Sprintf("evt-%d", seq) }

type TaskRef struct {
	ID        string `json:"id"`
	SourceKey string `json:"sourceKey"`
	Title     string `json:"title"`
	URL       string `json:"url,omitempty"`
}

type RunRef struct {
	ID      string  `json:"id"`
	Stage   string  `json:"stage,omitempty"`
	Round   int     `json:"round"`
	CostUSD float64 `json:"costUsd"`
	Turns   int     `json:"turns"`
	State   string  `json:"state"`
	PRURL   string  `json:"prUrl,omitempty"`
}

type ApprovalRef struct {
	ID        string     `json:"id"`
	Gate      string     `json:"gate"`
	Revision  int64      `json:"revision"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	Questions []string   `json:"questions,omitempty"`
	Role      string     `json:"role,omitempty"`
}

type Payload struct {
	EventID  string            `json:"eventId"`
	Seq      int64             `json:"seq"`
	Kind     domain.EventKind  `json:"kind"`
	At       time.Time         `json:"at"`
	Task     *TaskRef          `json:"task,omitempty"`
	Run      *RunRef           `json:"run,omitempty"`
	Approval *ApprovalRef      `json:"approval,omitempty"`
	Summary  string            `json:"summary,omitempty"`
	Links    map[string]string `json:"links"`
}
