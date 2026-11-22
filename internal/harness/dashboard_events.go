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
