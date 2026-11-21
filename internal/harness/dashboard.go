package harness

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"kode-stream/internal/harness/domain"
)

// The dashboard reads the harness through these views. They carry the
// domain fields and add the names the dashboard's screens use: a run with its
// current stage and active agent run, an agent run with its verdict and
// findings, approval context in plain words, settings as a flat list of
// sources, and events that carry the whole entity they are about. The bot
// routes keep the domain shapes.

// DashboardRoleOf names a role key the way the dashboard does: the tester's
// pool is "qa".
func DashboardRoleOf(key domain.RoleKey) string {
	if key == domain.RoleKeyQAFixer {
		return "qa"
	}
	return string(key)
}

// RoleKeyFromDashboard reads a dashboard role id; "qa" is the tester's pool.
func RoleKeyFromDashboard(role string) domain.RoleKey {
	if role == "qa" {
		return domain.RoleKeyQAFixer
	}
	return domain.RoleKey(role)
}

type FinishedOutside struct {
	By string     `json:"by,omitempty"`
	At *time.Time `json:"at,omitempty"`
}

type DashboardTask struct {
	domain.Task
	Milestone       string           `json:"milestone,omitempty"`
	Epic            string           `json:"epic,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
	FinishedOutside *FinishedOutside `json:"finishedOutside,omitempty"`
}

func dashboardTask(task domain.Task) DashboardTask {
	view := DashboardTask{Task: task, Milestone: task.Source.Milestone, Epic: task.Source.Epic, CreatedAt: task.DiscoveredAt}
	// A list carries neither the untrusted body nor the comments; the
	// detail view and the brief read them.
	view.Body, view.Comments = "", nil
	if task.Source.Labels == nil {
		view.Source.Labels = []string{}
	}
	if task.ScoreReasons == nil {
		view.ScoreReasons = []string{}
	}
	if task.DependsOn == nil {
		view.DependsOn = []string{}
	}
	// A task its source stopped listing was, in practice, closed there.
	if task.State == domain.TaskStale {
		at := task.UpdatedAt
		view.FinishedOutside = &FinishedOutside{At: &at}
	}
	return view
}

func dashboardTasks(tasks []domain.Task) []DashboardTask {
	out := make([]DashboardTask, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, dashboardTask(task))
	}
	return out
}

type DashboardAgentRun struct {
	domain.AgentRun
	Verdict  domain.Verdict   `json:"verdict,omitempty"`
	Blocking int              `json:"blocking"`
	Advisory int              `json:"advisory"`
	Summary  string           `json:"summary,omitempty"`
	Findings []domain.Finding `json:"findings,omitempty"`
}

func dashboardAgentRun(agentRun domain.AgentRun) DashboardAgentRun {
	view := DashboardAgentRun{AgentRun: agentRun}
	if report := agentRun.Report; report != nil {
		view.Verdict, view.Summary, view.Findings = report.Verdict, report.Summary, report.Findings
		for _, finding := range report.Findings {
			if finding.Severity == "blocking" {
				view.Blocking++
			} else {
				view.Advisory++
			}
		}
	}
	if agentRun.HarnessGate != nil && !agentRun.HarnessGate.Passed {
		view.Blocking++
	}
	return view
}

type DashboardRun struct {
	domain.Run
	CurrentStage   *domain.Stage      `json:"currentStage,omitempty"`
	ActiveAgentRun *DashboardAgentRun `json:"activeAgentRun,omitempty"`
	// RemoteBranch is the name the branch is pushed under.
	RemoteBranch string `json:"remoteBranch,omitempty"`
}

func (s *Service) dashboardRun(ctx context.Context, run domain.Run) DashboardRun {
	view := DashboardRun{Run: run}
	if s.worktrees != nil && run.Branch != "" {
		view.RemoteBranch = s.worktrees.RemoteBranch(run.Branch)
	}
	if run.CurrentStageID != "" {
		if stage, err := s.repository.GetStage(ctx, run.CurrentStageID); err == nil {
			view.CurrentStage = &stage
			if agentRuns, err := s.repository.ListAgentRuns(ctx, domain.AgentRunFilter{StageID: stage.ID, States: []domain.AgentRunState{domain.AgentRunning, domain.AgentQueued}, Limit: 10}); err == nil && len(agentRuns) > 0 {
				active := dashboardAgentRun(agentRuns[len(agentRuns)-1])
				if active.State == domain.AgentRunning {
					active.Progress = s.progressOf(active.ID)
				}
				view.ActiveAgentRun = &active
			}
		}
	}
	return view
}

// DashboardRuns lists runs with their current stage and active agent run.
func (s *Service) DashboardRuns(ctx context.Context, filter domain.RunFilter) ([]DashboardRun, error) {
	runs, err := s.Runs(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]DashboardRun, 0, len(runs))
	for _, run := range runs {
		out = append(out, s.dashboardRun(ctx, run))
	}
	return out, nil
}

type DashboardApprovalContext struct {
	Difficulty string         `json:"difficulty,omitempty"`
	Subtasks   int            `json:"subtasks,omitempty"`
	Rounds     int            `json:"rounds,omitempty"`
	Blocking   int            `json:"blocking,omitempty"`
	Advisory   int            `json:"advisory,omitempty"`
	CostUSD    float64        `json:"costUsd"`
	DurationMs int64          `json:"durationMs,omitempty"`
	Verdict    domain.Verdict `json:"verdict,omitempty"`
	Questions  []string       `json:"questions,omitempty"`
	Role       domain.Role    `json:"role,omitempty"`
	// Reporter and Draft belong to a clarify gate: who it waits for, and the
	// comment a person may post on the ticket to ask them.
	Reporter string `json:"reporter,omitempty"`
	Draft    string `json:"draft,omitempty"`
}

type DashboardApproval struct {
	domain.Approval
	Context DashboardApprovalContext `json:"context"`
}

// plainDifficulty reads the planner's rating as the dashboard's two words.
func plainDifficulty(difficulty string) string {
	switch difficulty {
	case "high":
		return "hard"
	case "low", "medium":
		return "easy"
	}
	return ""
}

func (s *Service) dashboardApproval(ctx context.Context, approval domain.Approval) DashboardApproval {
	view := DashboardApproval{Approval: approval, Context: DashboardApprovalContext{Difficulty: plainDifficulty(approval.Context.Difficulty), Subtasks: approval.Context.Subtasks, Rounds: approval.Context.Round, CostUSD: approval.Context.CostSoFarUSD, Questions: approval.Context.Questions, Role: approval.Context.Role, Reporter: approval.Context.Reporter, Draft: approval.Context.Draft}}
	if approval.RunID != "" {
		if run, err := s.repository.GetRun(ctx, approval.RunID); err == nil {
			view.Context.DurationMs, view.Context.Blocking, view.Context.Advisory = run.Totals.DurationMs, run.Totals.Blocking, run.Totals.Advisory
		}
	}
	if approval.Gate == domain.GatePlan {
		view.Context.Verdict = domain.VerdictApprove
	}
	return view
}

func (s *Service) DashboardApprovals(ctx context.Context, filter domain.ApprovalFilter) ([]DashboardApproval, error) {
	approvals, err := s.Approvals(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]DashboardApproval, 0, len(approvals))
	for _, approval := range approvals {
		out = append(out, s.dashboardApproval(ctx, approval))
	}
	return out, nil
}

// DashboardDecision maps a decided approval for the dashboard.
func (s *Service) DashboardDecision(ctx context.Context, approval domain.Approval) DashboardApproval {
	return s.dashboardApproval(ctx, approval)
}

type DashboardRunDetail struct {
	Run       DashboardRun        `json:"run"`
	Task      DashboardTask       `json:"task"`
	Stages    []domain.Stage      `json:"stages"`
	AgentRuns []DashboardAgentRun `json:"agentRuns"`
	Approvals []DashboardApproval `json:"approvals"`
}

func (s *Service) DashboardRunDetail(ctx context.Context, id string) (DashboardRunDetail, error) {
	if !s.Supported() {
		return DashboardRunDetail{}, domain.ErrNotFound
	}
	run, err := s.repository.GetRun(ctx, id)
	if err != nil {
		return DashboardRunDetail{}, err
	}
	task, err := s.repository.GetTask(ctx, run.TaskID)
	if err != nil {
		return DashboardRunDetail{}, err
	}
	stages, err := s.repository.ListStages(ctx, id)
	if err != nil {
		return DashboardRunDetail{}, err
	}
	agentRuns, err := s.repository.ListAgentRuns(ctx, domain.AgentRunFilter{RunID: id, Limit: 1000})
	if err != nil {
		return DashboardRunDetail{}, err
	}
	approvals, err := s.DashboardApprovals(ctx, domain.ApprovalFilter{RunID: id, Limit: 100})
	if err != nil {
		return DashboardRunDetail{}, err
	}
	detail := DashboardRunDetail{Run: s.dashboardRun(ctx, run), Task: dashboardTask(task), Stages: stages, AgentRuns: make([]DashboardAgentRun, 0, len(agentRuns)), Approvals: approvals}
	for _, agentRun := range agentRuns {
		view := dashboardAgentRun(agentRun)
		if agentRun.State == domain.AgentRunning {
			view.Progress = s.progressOf(agentRun.ID)
		}
		detail.AgentRuns = append(detail.AgentRuns, view)
	}
	return detail, nil
}

// DashboardAgentRunView is one agent run. A harness agent run is not a Kode
// Stream terminal session, so it never carries one.
type DashboardAgentRunView struct {
	AgentRun DashboardAgentRun `json:"agentRun"`
}

func (s *Service) DashboardAgentRun(ctx context.Context, id string) (DashboardAgentRunView, error) {
	agentRun, err := s.AgentRun(ctx, id)
	if err != nil {
		return DashboardAgentRunView{}, err
	}
	return DashboardAgentRunView{AgentRun: dashboardAgentRun(agentRun)}, nil
}

// --- status ---

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

// rolePolicyOf names the agent role whose policy a pool shows.
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

// DashboardRoleHealth reports one role after a pause or resume.
func DashboardRoleHealth(health RoleHealth) DashboardRole {
	return DashboardRole{Role: DashboardRoleOf(health.Role), Paused: health.Paused, Working: health.Busy, Slots: health.Slots, LastRunAt: health.LastActivityAt, LastError: health.LastError}
}

// --- cost ---

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

// --- settings ---

type DashboardSource struct {
	WorkspaceID string   `json:"workspaceId"`
	Kind        string   `json:"kind"`
	Enabled     bool     `json:"enabled"`
	Query       string   `json:"query"`
	Labels      []string `json:"labels,omitempty"`
	TokenEnvVar string   `json:"tokenEnvVar,omitempty"`
	// CheckReady and ReadyStatuses are a Jira source's readiness check: nil
	// CheckReady means off, and empty ReadyStatuses means READY.
	CheckReady    *bool      `json:"checkReady,omitempty"`
	ReadyStatuses []string   `json:"readyStatuses,omitempty"`
	LastCheckedAt *time.Time `json:"lastCheckedAt,omitempty"`
	LastFound     int        `json:"lastFound,omitempty"`
	LastError     string     `json:"lastError,omitempty"`
}

// DashboardWorkspace is one workspace's overrides. A nil or empty field
// follows the settings.
type DashboardWorkspace struct {
	WorkspaceID          string                               `json:"workspaceId"`
	Enabled              *bool                                `json:"enabled,omitempty"`
	AutoStart            *bool                                `json:"autoStart,omitempty"`
	RequireReview        *bool                                `json:"requireReview,omitempty"`
	AskQuestions         *bool                                `json:"askQuestions,omitempty"`
	VerificationCommands []string                             `json:"verificationCommands"`
	Scope                []string                             `json:"scope"`
	Models               map[domain.Role]domain.RoleModel     `json:"models"`
	Procedure            map[domain.Role]domain.ProcedureStep `json:"procedure"`
}

type DashboardRolePolicy struct {
	Engine    string `json:"engine"`
	Model     string `json:"model"`
	HardModel string `json:"hardModel,omitempty"`
	MaxTurns  int    `json:"maxTurns"`
}

type DashboardSettings struct {
	Revision      int64             `json:"revision"`
	Enabled       bool              `json:"enabled"`
	AutoStart     bool              `json:"autoStart"`
	RequireReview bool              `json:"requireReview"`
	AskQuestions  bool              `json:"askQuestions"`
	Slots         map[string]int    `json:"slots"`
	Paused        map[string]bool   `json:"paused"`
	Sources       []DashboardSource `json:"sources"`
	// Workspaces holds each workspace's own choices over these settings. A
	// save that omits it keeps the stored ones.
	Workspaces []DashboardWorkspace `json:"workspaces"`
	// ModelPolicy is the models a new workspace starts with. Once
	// WorkspaceModels is set, a save leaves it as it is.
	ModelPolicy      map[domain.Role]DashboardRolePolicy `json:"modelPolicy"`
	WorkspaceModels  bool                                `json:"workspaceModels"`
	ExpectedRevision *int64                              `json:"expectedRevision,omitempty"`
}

func (s *Service) DashboardSettings(ctx context.Context) (DashboardSettings, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return DashboardSettings{}, err
	}
	view := DashboardSettings{Revision: settings.Revision, WorkspaceModels: settings.WorkspaceModels, Enabled: settings.Enabled, AutoStart: settings.AutoStart, RequireReview: settings.RequireReview, AskQuestions: settings.QuestionGate(domain.Task{}),
		Slots:  map[string]int{"scout": 1, "sorter": 1, "planner": settings.Slots.Planner, "reviewer": settings.Slots.Reviewer, "coder": settings.Slots.Coder, "qa": settings.Slots.QAFixer},
		Paused: map[string]bool{}, Sources: []DashboardSource{}, Workspaces: []DashboardWorkspace{}, ModelPolicy: map[domain.Role]DashboardRolePolicy{}}
	for key, paused := range settings.Paused {
		if paused {
			view.Paused[DashboardRoleOf(key)] = true
		}
	}
	health := map[string]SourceHealth{}
	for _, source := range s.sourceHealth() {
		health[source.WorkspaceID+"\x00"+source.Kind] = source
	}
	add := func(source DashboardSource) {
		if known, ok := health[source.WorkspaceID+"\x00"+source.Kind]; ok {
			source.LastCheckedAt, source.LastFound, source.LastError = known.LastPolledAt, known.Added, known.LastError
		}
		view.Sources = append(view.Sources, source)
	}
	for _, workspace := range settings.Workspaces {
		if jira := workspace.Jira; jira != nil {
			add(DashboardSource{WorkspaceID: workspace.WorkspaceID, Kind: string(domain.SourceJira), Enabled: jira.Enabled, Query: jira.JQL, CheckReady: jira.CheckReady, ReadyStatuses: jira.ReadyStatuses})
		}
		if github := workspace.GitHub; github != nil {
			add(DashboardSource{WorkspaceID: workspace.WorkspaceID, Kind: string(domain.SourceGitHub), Enabled: github.Enabled, Query: github.Repo, Labels: github.Labels, TokenEnvVar: github.TokenEnvVar})
		}
		if bitbucket := workspace.Bitbucket; bitbucket != nil {
			add(DashboardSource{WorkspaceID: workspace.WorkspaceID, Kind: string(domain.SourceBitbucket), Enabled: bitbucket.Enabled, Query: bitbucket.Workspace + "/" + bitbucket.Repo, TokenEnvVar: bitbucket.TokenEnvVar})
		}
		if local := workspace.Local; local != nil {
			add(DashboardSource{WorkspaceID: workspace.WorkspaceID, Kind: string(domain.SourceLocal), Enabled: local.Enabled, Query: local.Dir})
		}
	}
	for _, workspace := range settings.Workspaces {
		view.Workspaces = append(view.Workspaces, DashboardWorkspace{WorkspaceID: workspace.WorkspaceID, Enabled: workspace.Enabled, AutoStart: workspace.AutoStart, RequireReview: workspace.RequireReview, AskQuestions: workspace.AskQuestions,
			VerificationCommands: nonNil(workspace.VerificationCommands), Scope: nonNil(workspace.Scope), Models: orEmpty(workspace.Models), Procedure: orEmpty(workspace.Procedure)})
	}
	for _, role := range domain.Roles {
		policy := settings.ModelPolicy.Roles[role]
		view.ModelPolicy[role] = DashboardRolePolicy{Engine: policy.Engine, Model: policy.Model, HardModel: policy.HighDifficultyModel, MaxTurns: policy.MaxTurns}
	}
	return view, nil
}

// SaveDashboardSettings lays the dashboard's settings over the stored ones:
// what the dashboard does not show (scope, verification commands, tool
// lists, ranker, intervals) keeps its stored value.
func (s *Service) SaveDashboardSettings(ctx context.Context, view DashboardSettings, actor domain.Actor) (DashboardSettings, error) {
	if err := s.mutable(); err != nil {
		return DashboardSettings{}, err
	}
	settings := s.settingsNow(ctx)
	expected := view.Revision
	if view.ExpectedRevision != nil {
		expected = *view.ExpectedRevision
	}
	settings.Enabled, settings.AutoStart, settings.RequireReview = view.Enabled, view.AutoStart, view.RequireReview
	ask := view.AskQuestions
	settings.AskQuestions = &ask
	slot := func(key string, current int) int {
		if value, ok := view.Slots[key]; ok && value > 0 {
			return value
		}
		return current
	}
	settings.Slots = domain.Slots{Planner: slot("planner", settings.Slots.Planner), Reviewer: slot("reviewer", settings.Slots.Reviewer), Coder: slot("coder", settings.Slots.Coder), QAFixer: slot("qa", settings.Slots.QAFixer)}
	if view.Paused != nil {
		settings.Paused = map[domain.RoleKey]bool{}
		for role, paused := range view.Paused {
			key := RoleKeyFromDashboard(role)
			if !domain.ValidRoleKey(key) {
				return DashboardSettings{}, domain.Invalid("unknown role %q in paused", role)
			}
			if paused {
				settings.Paused[key] = true
			}
		}
	}
	if view.Sources != nil {
		if err := applySources(&settings, view.Sources); err != nil {
			return DashboardSettings{}, err
		}
	}
	if view.Workspaces != nil {
		applyWorkspaces(&settings, view.Workspaces)
	}
	// Models are each workspace's own once moved; before that, the defaults'
	// still decide the workspaces that set none.
	if !settings.WorkspaceModels {
		for role, policy := range view.ModelPolicy {
			current, ok := settings.ModelPolicy.Roles[role]
			if !ok {
				return DashboardSettings{}, domain.Invalid("unknown role %q in modelPolicy", role)
			}
			current.Engine, current.Model, current.MaxTurns = policy.Engine, policy.Model, policy.MaxTurns
			if role == domain.RoleCoder {
				current.HighDifficultyModel = policy.HardModel
			}
			settings.ModelPolicy.Roles[role] = current
		}
	}
	if _, err := s.SaveSettings(ctx, settings, expected, actor); err != nil {
		return DashboardSettings{}, err
	}
	return s.DashboardSettings(ctx)
}

// applySources sets each workspace's sources from the dashboard's list. A
// workspace's other settings are kept; a source it no longer lists is
// removed.
func applySources(settings *domain.Settings, sources []DashboardSource) error {
	listed := map[string]map[string]DashboardSource{}
	order := []string{}
	for _, source := range sources {
		if strings.TrimSpace(source.WorkspaceID) == "" {
			return domain.Invalid("a source needs its workspaceId")
		}
		if listed[source.WorkspaceID] == nil {
			listed[source.WorkspaceID] = map[string]DashboardSource{}
			order = append(order, source.WorkspaceID)
		}
		listed[source.WorkspaceID][source.Kind] = source
	}
	index := map[string]int{}
	for i, workspace := range settings.Workspaces {
		index[workspace.WorkspaceID] = i
	}
	for _, id := range order {
		if _, ok := index[id]; !ok {
			settings.Workspaces = append(settings.Workspaces, domain.WorkspaceSettings{WorkspaceID: id})
			index[id] = len(settings.Workspaces) - 1
		}
	}
	for i := range settings.Workspaces {
		workspace := &settings.Workspaces[i]
		kinds := listed[workspace.WorkspaceID]
		workspace.Jira, workspace.GitHub, workspace.Local = nil, nil, nil
		previousBitbucket := workspace.Bitbucket
		workspace.Bitbucket = nil
		for kind, source := range kinds {
			query := strings.TrimSpace(source.Query)
			switch domain.SourceKind(kind) {
			case domain.SourceJira:
				workspace.Jira = &domain.JiraSourceSettings{Enabled: source.Enabled, JQL: query, CheckReady: source.CheckReady, ReadyStatuses: source.ReadyStatuses}
			case domain.SourceGitHub:
				workspace.GitHub = &domain.GitHubSourceSettings{Enabled: source.Enabled, Repo: query, Labels: source.Labels, TokenEnvVar: source.TokenEnvVar}
			case domain.SourceBitbucket:
				owner, repo, _ := strings.Cut(query, "/")
				bitbucket := &domain.BitbucketSourceSettings{Enabled: source.Enabled, Workspace: owner, Repo: repo, TokenEnvVar: source.TokenEnvVar, Cloud: true}
				if previousBitbucket != nil {
					bitbucket.Cloud, bitbucket.BaseURL = previousBitbucket.Cloud, previousBitbucket.BaseURL
				}
				workspace.Bitbucket = bitbucket
			case domain.SourceLocal:
				workspace.Local = &domain.LocalSourceSettings{Enabled: source.Enabled, Dir: query}
			default:
				return domain.Invalid("source kind %q must be jira, github, bitbucket or local", kind)
			}
		}
	}
	return nil
}

// applyWorkspaces sets each listed workspace's overrides; a workspace it
// does not list keeps its own.
func applyWorkspaces(settings *domain.Settings, workspaces []DashboardWorkspace) {
	for _, view := range workspaces {
		if strings.TrimSpace(view.WorkspaceID) == "" {
			continue
		}
		index := -1
		for i := range settings.Workspaces {
			if settings.Workspaces[i].WorkspaceID == view.WorkspaceID {
				index = i
			}
		}
		if index < 0 {
			settings.Workspaces = append(settings.Workspaces, domain.WorkspaceSettings{WorkspaceID: view.WorkspaceID})
			index = len(settings.Workspaces) - 1
		}
		workspace := &settings.Workspaces[index]
		workspace.Enabled, workspace.AutoStart, workspace.RequireReview, workspace.AskQuestions = view.Enabled, view.AutoStart, view.RequireReview, view.AskQuestions
		workspace.VerificationCommands = cleanCommands(view.VerificationCommands)
		workspace.Scope = cleanCommands(view.Scope)
		workspace.Models, workspace.Procedure = view.Models, view.Procedure
	}
}

func nonNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

func orEmpty[K comparable, V any](values map[K]V) map[K]V {
	if values == nil {
		return map[K]V{}
	}
	return values
}

// cleanCommands drops blank test commands and trims the rest.
func cleanCommands(commands []string) []string {
	out := []string{}
	for _, command := range commands {
		if command = strings.TrimSpace(command); command != "" {
			out = append(out, command)
		}
	}
	return out
}

// --- events ---

// DashboardEvent describes a stored or live event for the dashboard's feed,
// with the whole entity it is about. ok is false for an event the dashboard
// does not show.
func (s *Service) DashboardEvent(ctx context.Context, event domain.Event) (map[string]any, bool) {
	out := map[string]any{"seq": event.Seq, "type": event.Kind, "at": event.At}
	switch event.Kind {
	case domain.EventTaskDiscovered, domain.EventTaskUpdated, domain.EventTaskRanked, domain.EventTaskPinned:
		task, err := s.repository.GetTask(ctx, event.TaskID)
		if err != nil {
			return nil, false
		}
		out["task"] = dashboardTask(task)
	case domain.EventTaskRemoved:
		out["taskId"] = event.TaskID
	case domain.EventRunStarted, domain.EventRunBlocked, domain.EventRunFinished:
		run, err := s.repository.GetRun(ctx, event.RunID)
		if err != nil {
			return nil, false
		}
		out["run"] = s.dashboardRun(ctx, run)
	case domain.EventStageStarted, domain.EventStageRound, domain.EventStageFinished:
		if event.StageID == "" {
			return nil, false
		}
		stage, err := s.repository.GetStage(ctx, event.StageID)
		if err != nil {
			return nil, false
		}
		out["stage"] = stage
	case domain.EventAgentStarted, domain.EventAgentProgress, domain.EventAgentFinished:
		agentRun, err := s.repository.GetAgentRun(ctx, event.AgentRunID)
		if err != nil {
			return nil, false
		}
		view := dashboardAgentRun(agentRun)
		if event.Kind == domain.EventAgentProgress {
			var fields struct {
				Progress string `json:"progress"`
				Turns    int    `json:"turns"`
			}
			_ = json.Unmarshal(event.Payload, &fields)
			view.Progress = fields.Progress
			if fields.Turns > view.Turns {
				view.Turns = fields.Turns
			}
		}
		out["agentRun"] = view
	case domain.EventApprovalRequested, domain.EventApprovalDecided:
		approval, err := s.repository.GetApproval(ctx, event.ApprovalID)
		if err != nil {
			return nil, false
		}
		out["approval"] = s.dashboardApproval(ctx, approval)
	case domain.EventRolePaused, domain.EventRoleResumed:
		var fields struct {
			Role domain.RoleKey `json:"role"`
		}
		_ = json.Unmarshal(event.Payload, &fields)
		status, err := s.DashboardStatus(ctx)
		if err != nil {
			return nil, false
		}
		found := false
		for _, role := range status.Roles {
			if role.Role == DashboardRoleOf(fields.Role) {
				out["role"], found = role, true
			}
		}
		if !found {
			return nil, false
		}
	case domain.EventNotificationFailed:
		var fields struct {
			EventID string `json:"eventId"`
			Error   string `json:"error"`
		}
		_ = json.Unmarshal(event.Payload, &fields)
		out["message"] = "The bot was not told about " + fields.EventID + ": " + fields.Error
	default:
		return nil, false
	}
	// A run or a gate changes its task too (its state, its run, whether it
	// needs a person), and no task event follows, so the task travels along.
	switch event.Kind {
	case domain.EventRunStarted, domain.EventRunBlocked, domain.EventRunFinished, domain.EventApprovalRequested, domain.EventApprovalDecided:
		if event.TaskID != "" {
			if task, err := s.repository.GetTask(ctx, event.TaskID); err == nil {
				out["task"] = dashboardTask(task)
			}
		}
	}
	return out, true
}

// --- changes ---

// DashboardChanges is a run's changes in the shape of a session's changes.
type DashboardChanges struct {
	SessionID    string                 `json:"sessionId"`
	LaunchCommit string                 `json:"launchCommit"`
	HeadCommit   string                 `json:"headCommit"`
	HeadMoved    int                    `json:"headMoved"`
	Files        []DashboardChangedFile `json:"files"`
	Additions    int                    `json:"additions"`
	Deletions    int                    `json:"deletions"`
	Truncated    bool                   `json:"truncated"`
	Availability string                 `json:"availability"`
	Reason       string                 `json:"reason,omitempty"`
	Branch       string                 `json:"branch"`
	// From says where the files were read: "worktree" or "branch".
	From string `json:"from,omitempty"`
}

type DashboardChangedFile struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

func (s *Service) DashboardChanges(ctx context.Context, runID string) (DashboardChanges, error) {
	changes, err := s.Changes(ctx, runID)
	if err != nil {
		return DashboardChanges{}, err
	}
	view := DashboardChanges{SessionID: runID, LaunchCommit: changes.BaseCommit, Files: []DashboardChangedFile{}, Availability: "available", Branch: changes.Branch, From: changes.From}
	if !changes.Available {
		view.Availability, view.Reason = "unavailable", "workspace_missing"
	}
	codes := map[string]string{"added": "A", "deleted": "D", "modified": "M"}
	sort.Slice(changes.Files, func(i, j int) bool { return changes.Files[i].Path < changes.Files[j].Path })
	for _, file := range changes.Files {
		view.Files = append(view.Files, DashboardChangedFile{Path: file.Path, Status: codes[file.Status], Additions: file.Additions, Deletions: file.Deletions})
		view.Additions += file.Additions
		view.Deletions += file.Deletions
	}
	return view, nil
}

// DashboardTasks maps tasks for the dashboard, without their bodies.
func DashboardTasks(tasks []domain.Task) []DashboardTask { return dashboardTasks(tasks) }

// DashboardTaskWithBody maps one task with its untrusted body, for its
// detail view.
func DashboardTaskWithBody(task domain.Task) DashboardTask {
	view := dashboardTask(task)
	view.Body, view.Comments = task.Body, task.Comments
	return view
}
