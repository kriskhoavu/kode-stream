package ai

import (
	"context"
	"sort"
)

// Session list scopes for SessionRecordsAcrossWorkspaces.
const (
	SessionScopeActive = "active"
	SessionScopeAll    = "all"
)

// maxGlobalSessionRows bounds the cross-workspace session list, mirroring the
// cap the Git since-commit readers apply to their own listings.
const maxGlobalSessionRows = 200

// GlobalSessionRow is one row of the cross-workspace session list: a session
// record view annotated with what a workspace-scoped list does not carry.
type GlobalSessionRow struct {
	Record         SessionRecordView `json:"record"`
	WorkspaceName  string            `json:"workspaceName"`
	CheckoutBranch string            `json:"checkoutBranch,omitempty"`
	PlanTitle      string            `json:"planTitle,omitempty"`
	ChangesCount   *int              `json:"changesCount,omitempty"`
}

// SessionRecordsAcrossWorkspaces lists session records for every registered
// workspace mayRead allows, annotated with workspace name, best-effort
// checkout branch, best-effort plan title, and (scope=active only) a changes
// count. A workspace mayRead denies is simply absent from the result, never a
// reported error -- the same oracle-avoidance the AI controller's workspace
// authorization already follows. Rows are newest first and capped at
// maxGlobalSessionRows.
func (s *Service) SessionRecordsAcrossWorkspaces(ctx context.Context, scope string, mayRead func(workspaceID string) bool) ([]GlobalSessionRow, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if s.records == nil || s.launch == nil || s.launch.registry == nil {
		return []GlobalSessionRow{}, false, nil
	}
	workspaces, err := s.launch.registry.List()
	if err != nil {
		return nil, false, err
	}
	active := scope == SessionScopeActive
	rows := make([]GlobalSessionRow, 0, maxGlobalSessionRows)
	for _, workspace := range workspaces {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if mayRead != nil && !mayRead(workspace.ID) {
			continue
		}
		views, err := s.SessionRecordsContext(ctx, workspace.ID, "")
		if err != nil {
			return nil, false, err
		}
		if len(views) == 0 {
			continue
		}
		// Resolve the checkout branch once per workspace rather than once per
		// session row: it is the same answer for every row from this
		// workspace, and a workspace can carry many session rows.
		var checkoutBranch string
		if s.branches != nil {
			if branch, err := s.branches.CurrentBranch(workspace.Path); err == nil {
				checkoutBranch = branch
			}
		}
		for _, view := range views {
			if active && !(view.Live && (view.State == StateStarting || view.State == StateRunning)) {
				continue
			}
			row := GlobalSessionRow{Record: view, WorkspaceName: workspace.Name, CheckoutBranch: checkoutBranch}
			if view.PlanRef != nil && s.launch.index != nil {
				if item, found, err := s.launch.index.Get(view.PlanRef.ItemID); err == nil && found {
					row.PlanTitle = item.Title
				}
			}
			rows = append(rows, row)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Record.StartedAt.Equal(rows[j].Record.StartedAt) {
			return rows[i].Record.ID < rows[j].Record.ID
		}
		return rows[i].Record.StartedAt.After(rows[j].Record.StartedAt)
	})
	truncated := false
	if len(rows) > maxGlobalSessionRows {
		rows = rows[:maxGlobalSessionRows]
		truncated = true
	}
	// Only annotate the changes count for the rows that survived the cap:
	// SessionChanges runs Git commands, so bounding it to at most
	// maxGlobalSessionRows keeps the work proportional to what is returned
	// instead of to however many active sessions exist.
	if active {
		for i := range rows {
			if err := ctx.Err(); err != nil {
				return nil, false, err
			}
			if changes, err := s.SessionChanges(ctx, rows[i].Record.ID); err == nil && changes.Availability == SessionChangesAvailable {
				count := len(changes.Files)
				rows[i].ChangesCount = &count
			}
		}
	}
	return rows, truncated, nil
}
