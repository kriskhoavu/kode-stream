package models

import "time"

type AuditStatus string

const (
	AuditStatusSuccess AuditStatus = "success"
	AuditStatusBlocked AuditStatus = "blocked"
	AuditStatusFailed  AuditStatus = "failed"
)

type AuditEvent struct {
	ID          string      `json:"id" yaml:"id"`
	Time        time.Time   `json:"time" yaml:"time"`
	OwnerUserID string      `json:"ownerUserId,omitempty" yaml:"ownerUserId,omitempty"`
	ActorUserID string      `json:"actorUserId,omitempty" yaml:"actorUserId,omitempty"`
	WorkspaceID string      `json:"workspaceId,omitempty" yaml:"workspaceId,omitempty"`
	ItemID      string      `json:"itemId,omitempty" yaml:"itemId,omitempty"`
	Operation   string      `json:"operation" yaml:"operation"`
	Status      AuditStatus `json:"status" yaml:"status"`
	Message     string      `json:"message" yaml:"message"`
	Paths       []string    `json:"paths" yaml:"paths"`
	DurationMS  int64       `json:"durationMs" yaml:"durationMs"`
	Error       string      `json:"error,omitempty" yaml:"error,omitempty"`
}

type HealthStatus string

const (
	HealthStatusOK      HealthStatus = "ok"
	HealthStatusWarning HealthStatus = "warning"
	HealthStatusFailed  HealthStatus = "failed"
)

type HealthCheck struct {
	Name         string       `json:"name" yaml:"name"`
	Status       HealthStatus `json:"status" yaml:"status"`
	Message      string       `json:"message" yaml:"message"`
	RecoveryHint string       `json:"recoveryHint,omitempty" yaml:"recoveryHint,omitempty"`
}

type WorkspaceHealth struct {
	WorkspaceID string        `json:"workspaceId" yaml:"workspaceId"`
	CheckedAt   time.Time     `json:"checkedAt" yaml:"checkedAt"`
	Checks      []HealthCheck `json:"checks" yaml:"checks"`
	Summary     HealthStatus  `json:"summary" yaml:"summary"`
}

type SafetyCheck struct {
	OK           bool   `json:"ok" yaml:"ok"`
	Message      string `json:"message,omitempty" yaml:"message,omitempty"`
	RecoveryHint string `json:"recoveryHint,omitempty" yaml:"recoveryHint,omitempty"`
}

type SearchQuery struct {
	Text        string   `json:"q" yaml:"q"`
	WorkspaceID string   `json:"workspaceId,omitempty" yaml:"workspaceId,omitempty"`
	Types       []string `json:"types,omitempty" yaml:"types,omitempty"`
	Limit       int      `json:"limit,omitempty" yaml:"limit,omitempty"`
}

type SearchResult struct {
	ID          string `json:"id" yaml:"id"`
	Type        string `json:"type" yaml:"type"`
	Title       string `json:"title" yaml:"title"`
	Subtitle    string `json:"subtitle" yaml:"subtitle"`
	Context     string `json:"context" yaml:"context"`
	WorkspaceID string `json:"workspaceId,omitempty" yaml:"workspaceId,omitempty"`
	ItemID      string `json:"itemId,omitempty" yaml:"itemId,omitempty"`
	Route       string `json:"route" yaml:"route"`
	Score       int    `json:"score" yaml:"score"`
}

type SavedFilter struct {
	ID          string         `json:"id" yaml:"id"`
	OwnerUserID string         `json:"ownerUserId,omitempty" yaml:"ownerUserId,omitempty"`
	Name        string         `json:"name" yaml:"name"`
	Route       string         `json:"route" yaml:"route"`
	WorkspaceID string         `json:"workspaceId,omitempty" yaml:"workspaceId,omitempty"`
	Filters     map[string]any `json:"filters" yaml:"filters"`
	CreatedAt   time.Time      `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt" yaml:"updatedAt"`
}

type RecentItem struct {
	ItemID      string    `json:"itemId" yaml:"itemId"`
	OwnerUserID string    `json:"ownerUserId,omitempty" yaml:"ownerUserId,omitempty"`
	WorkspaceID string    `json:"workspaceId" yaml:"workspaceId"`
	Title       string    `json:"title" yaml:"title"`
	Subtitle    string    `json:"subtitle,omitempty" yaml:"subtitle,omitempty"`
	Route       string    `json:"route" yaml:"route"`
	OpenedAt    time.Time `json:"openedAt" yaml:"openedAt"`
}
