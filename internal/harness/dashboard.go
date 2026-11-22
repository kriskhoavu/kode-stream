package harness

import (
	"context"
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

// --- cost ---

// --- settings ---

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

// --- events ---

// --- changes ---

// DashboardTasks maps tasks for the dashboard, without their bodies.
func DashboardTasks(tasks []domain.Task) []DashboardTask { return dashboardTasks(tasks) }

// DashboardTaskWithBody maps one task with its untrusted body, for its
// detail view.
func DashboardTaskWithBody(task domain.Task) DashboardTask {
	view := dashboardTask(task)
	view.Body, view.Comments = task.Body, task.Comments
	return view
}
