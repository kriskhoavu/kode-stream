package harness

import (
	"context"
	"time"

	"kode-stream/internal/harness/domain"
)

type DashboardRole struct {
	Role      string     `json:"role"`
	Paused    bool       `json:"paused"`
	Working   int        `json:"working"`
	Slots     int        `json:"slots"`
	Queued    int        `json:"queued"`
	LastRunAt *time.Time `json:"lastRunAt,omitempty"`
	LastError string     `json:"lastError,omitempty"`
	Found     int        `json:"found,omitempty"`
	Model     string     `json:"model,omitempty"`
	Engine    string     `json:"engine,omitempty"`
}

type DashboardEngine struct {
	ID        string   `json:"id"`
	Available bool     `json:"available"`
	Version   string   `json:"version,omitempty"`
	Path      string   `json:"path,omitempty"`
	Limits    []string `json:"limits"`
}

type DashboardStatus struct {
	Enabled          bool              `json:"enabled"`
	StorageSupported bool              `json:"storageSupported"`
	Mode             string            `json:"mode"`
	Running          bool              `json:"running"`
	AutoStart        bool              `json:"autoStart"`
	AskQuestions     bool              `json:"askQuestions"`
	WaitingCount     int               `json:"waitingCount"`
	LastEventSeq     int64             `json:"lastEventSeq"`
	Roles            []DashboardRole   `json:"roles"`
	Engines          []DashboardEngine `json:"engines"`
	Sources          []SourceHealth    `json:"sources"`
	Counts           StatusCounts      `json:"counts"`
	CostTodayUSD     float64           `json:"costTodayUsd"`
	Hint             string            `json:"hint,omitempty"`
}

var rolePolicyOf = map[domain.RoleKey]domain.Role{
	domain.RoleKeyPlanner:  domain.RolePlanner,
	domain.RoleKeyReviewer: domain.RoleCodeReviewer,
	domain.RoleKeyCoder:    domain.RoleCoder,
	domain.RoleKeyQAFixer:  domain.RoleQAFixer,
}

func (s *Service) DashboardStatus(ctx context.Context) (DashboardStatus, error) {
	status, err := s.Status(ctx)
	view := DashboardStatus{Enabled: status.Enabled, StorageSupported: status.StorageSupported, Mode: status.Mode, Running: status.Running, AutoStart: status.AutoStart, AskQuestions: status.AskQuestions, WaitingCount: status.Counts.WaitingApprovals, LastEventSeq: status.LastEventSeq, Roles: []DashboardRole{}, Engines: []DashboardEngine{}, Sources: status.Sources, Counts: status.Counts, CostTodayUSD: status.CostTodayUSD, Hint: status.Hint}
	if err != nil {
		return view, err
	}
	for _, engine := range status.Engines {
		view.Engines = append(view.Engines, DashboardEngine{ID: engine.ID, Available: engine.Available, Version: engine.Version, Path: engine.Path, Limits: engine.Unsupported})
	}
	if !s.Supported() {
		return view, nil
	}
	settings := s.settingsNow(ctx)
	queued := map[domain.RoleKey]int{}
	if waiting, err := s.repository.ListAgentRuns(ctx, domain.AgentRunFilter{States: []domain.AgentRunState{domain.AgentQueued}, Limit: 1000}); err == nil {
		for _, agentRun := range waiting {
			queued[domain.PoolOf(agentRun.Role)]++
		}
	}
	for _, health := range status.Roles {
		role := DashboardRole{Role: DashboardRoleOf(health.Role), Paused: health.Paused, Working: health.Busy, Slots: health.Slots, Queued: queued[health.Role], LastRunAt: health.LastActivityAt, LastError: health.LastError}
		switch health.Role {
		case domain.RoleKeyScout:
			for _, source := range status.Sources {
				role.Found += source.Added
			}
		case domain.RoleKeySorter:
			role.Queued = status.Counts.Queued
		default:
			policy := settings.ModelPolicy.Roles[rolePolicyOf[health.Role]]
			role.Engine, role.Model = policy.Engine, policy.Model
		}
		view.Roles = append(view.Roles, role)
	}
	return view, nil
}

func DashboardRoleHealth(health RoleHealth) DashboardRole {
	return DashboardRole{Role: DashboardRoleOf(health.Role), Paused: health.Paused, Working: health.Busy, Slots: health.Slots, LastRunAt: health.LastActivityAt, LastError: health.LastError}
}

type DashboardCostGroup struct {
	Key        string  `json:"key"`
	Label      string  `json:"label"`
	CostUSD    float64 `json:"costUsd"`
	AgentRuns  int     `json:"agentRuns"`
	Turns      int     `json:"turns"`
	DurationMs int64   `json:"durationMs"`
}

type DashboardCost struct {
	GroupBy   domain.CostGroup     `json:"groupBy"`
	From      time.Time            `json:"from"`
	To        time.Time            `json:"to"`
	TotalUSD  float64              `json:"totalUsd"`
	AgentRuns int                  `json:"agentRuns"`
	Groups    []DashboardCostGroup `json:"groups"`
}

func DashboardCostOf(summary domain.CostSummary) DashboardCost {
	view := DashboardCost{GroupBy: summary.GroupBy, From: summary.From, To: summary.To, TotalUSD: summary.TotalUSD, Groups: []DashboardCostGroup{}}
	for _, row := range summary.Rows {
		view.AgentRuns += row.AgentRuns
		view.Groups = append(view.Groups, DashboardCostGroup{Key: row.Key, Label: row.Label, CostUSD: row.CostUSD, AgentRuns: row.AgentRuns, Turns: row.Turns})
	}
	return view
}
