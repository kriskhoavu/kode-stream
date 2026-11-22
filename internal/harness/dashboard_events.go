package harness

import (
	"context"
	"encoding/json"
	"sort"

	"kode-stream/internal/harness/domain"
)

// DashboardEvent describes a stored or live event for the dashboard's feed,
// with the whole entity it is about. ok is false for an event the dashboard
// does not show.
func (s *Service) DashboardEvent(ctx context.Context, event domain.Event) (map[string]any, bool) {
	describe, shown := dashboardDescribers[event.Kind]
	if !shown {
		return nil, false
	}
	out := map[string]any{"seq": event.Seq, "type": event.Kind, "at": event.At}
	if !describe(s, ctx, event, out) {
		return nil, false
	}
	// A run or a gate changes its task too (its state, its run, whether it
	// needs a person), and no task event follows, so the task travels along.
	if carriesTask[event.Kind] && event.TaskID != "" {
		if task, err := s.repository.GetTask(ctx, event.TaskID); err == nil {
			out["task"] = dashboardTask(task)
		}
	}
	return out, true
}

// dashboardDescriber adds what one kind of event is about to out. It answers
// false when the entity is gone or the event names none.
type dashboardDescriber func(s *Service, ctx context.Context, event domain.Event, out map[string]any) bool

var dashboardDescribers = func() map[domain.EventKind]dashboardDescriber {
	describers := map[domain.EventKind]dashboardDescriber{}
	add := func(describe dashboardDescriber, kinds ...domain.EventKind) {
		for _, kind := range kinds {
			describers[kind] = describe
		}
	}
	add(describeTask, domain.EventTaskDiscovered, domain.EventTaskUpdated, domain.EventTaskRanked, domain.EventTaskPinned)
	add(describeRemovedTask, domain.EventTaskRemoved)
	add(describeRun, domain.EventRunStarted, domain.EventRunBlocked, domain.EventRunFinished)
	add(describeStage, domain.EventStageStarted, domain.EventStageRound, domain.EventStageFinished)
	add(describeAgentRun, domain.EventAgentStarted, domain.EventAgentProgress, domain.EventAgentFinished)
	add(describeApproval, domain.EventApprovalRequested, domain.EventApprovalDecided)
	add(describeRole, domain.EventRolePaused, domain.EventRoleResumed)
	add(describeNotificationFailure, domain.EventNotificationFailed)
	return describers
}()

var carriesTask = map[domain.EventKind]bool{
	domain.EventRunStarted: true, domain.EventRunBlocked: true, domain.EventRunFinished: true,
	domain.EventApprovalRequested: true, domain.EventApprovalDecided: true,
}

func describeTask(s *Service, ctx context.Context, event domain.Event, out map[string]any) bool {
	task, err := s.repository.GetTask(ctx, event.TaskID)
	if err != nil {
		return false
	}
	out["task"] = dashboardTask(task)
	return true
}

func describeRemovedTask(_ *Service, _ context.Context, event domain.Event, out map[string]any) bool {
	out["taskId"] = event.TaskID
	return true
}

func describeRun(s *Service, ctx context.Context, event domain.Event, out map[string]any) bool {
	run, err := s.repository.GetRun(ctx, event.RunID)
	if err != nil {
		return false
	}
	out["run"] = s.dashboardRun(ctx, run)
	return true
}

func describeStage(s *Service, ctx context.Context, event domain.Event, out map[string]any) bool {
	if event.StageID == "" {
		return false
	}
	stage, err := s.repository.GetStage(ctx, event.StageID)
	if err != nil {
		return false
	}
	out["stage"] = stage
	return true
}

func describeAgentRun(s *Service, ctx context.Context, event domain.Event, out map[string]any) bool {
	agentRun, err := s.repository.GetAgentRun(ctx, event.AgentRunID)
	if err != nil {
		return false
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
	return true
}

func describeApproval(s *Service, ctx context.Context, event domain.Event, out map[string]any) bool {
	approval, err := s.repository.GetApproval(ctx, event.ApprovalID)
	if err != nil {
		return false
	}
	out["approval"] = s.dashboardApproval(ctx, approval)
	return true
}

func describeRole(s *Service, ctx context.Context, event domain.Event, out map[string]any) bool {
	var fields struct {
		Role domain.RoleKey `json:"role"`
	}
	_ = json.Unmarshal(event.Payload, &fields)
	status, err := s.DashboardStatus(ctx)
	if err != nil {
		return false
	}
	found := false
	for _, role := range status.Roles {
		if role.Role == DashboardRoleOf(fields.Role) {
			out["role"], found = role, true
		}
	}
	return found
}

func describeNotificationFailure(_ *Service, _ context.Context, event domain.Event, out map[string]any) bool {
	var fields struct {
		EventID string `json:"eventId"`
		Error   string `json:"error"`
	}
	_ = json.Unmarshal(event.Payload, &fields)
	out["message"] = "The bot was not told about " + fields.EventID + ": " + fields.Error
	return true
}

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
