package canvas

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"kode-stream/internal/ai"
	"kode-stream/internal/common/models"
)

func boundedPlanItems(ctx context.Context, items []models.ItemSummary) []models.ItemSummary {
	result := make([]models.ItemSummary, 0, min(len(items), maxProjectionCandidates))
	for _, item := range items {
		if ctx.Err() != nil || len(result) >= maxProjectionCandidates {
			break
		}
		if canvasPlanService(item) != "" {
			result = append(result, item)
		}
	}
	return result
}

func canvasPlanItems(items []models.ItemSummary) []models.ItemSummary {
	result := make([]models.ItemSummary, 0, len(items))
	for _, item := range items {
		if canvasPlanService(item) != "" {
			result = append(result, item)
		}
	}
	return result
}

func canvasPlanService(item models.ItemSummary) string {
	return canvasPlanServiceFromPath(item.ItemPath)
}

func canvasPathKey(itemPath string) string {
	return strings.Trim(strings.ReplaceAll(itemPath, `\`, "/"), "/")
}

func canvasPlanServiceFromPath(itemPath string) string {
	parts := strings.Split(strings.Trim(strings.ReplaceAll(itemPath, `\`, "/"), "/"), "/")
	if len(parts) != 3 || parts[0] != "plans" || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		return ""
	}
	return parts[1]
}

// ephemeralSessionNodes projects every live session that has no placement but
// whose plan is on the Canvas, so a session started from a card shows up next
// to its plan without anyone placing it. It marks each in placed so the session
// is not offered as unplaced as well.
func ephemeralSessionNodes(nodes []ProjectedNode, sessions []ai.SessionRecordView, placed map[string]bool) []ProjectedNode {
	plans := map[string]ProjectedNode{}
	for _, node := range nodes {
		if node.Kind == EntityPlan && node.State == NodeResolved && node.Plan != nil {
			plans[node.Plan.ItemID] = node
		}
	}
	perPlan := map[string]int{}
	result := []ProjectedNode{}
	for _, session := range sessions {
		if !session.Live || session.PlanRef == nil || placed[sessionNodeID(session.ID)] {
			continue
		}
		plan, ok := plans[session.PlanRef.ItemID]
		if !ok {
			continue
		}
		slot := perPlan[plan.ID]
		perPlan[plan.ID]++
		placed[ephemeralMarker(session.ID)] = true
		result = append(result, ProjectedNode{
			ID: sessionNodeID(session.ID), Kind: EntitySession, State: NodeResolved, EntityRef: sessionRef(session),
			Position:  Position{X: plan.Position.X + ephemeralPlanWidth + ephemeralGutter + float64(slot)*(ephemeralSessionWidth+ephemeralGutter), Y: plan.Position.Y},
			Size:      &Size{Width: ephemeralSessionWidth, Height: ephemeralSessionHeight},
			Session:   &SessionNode{Record: session},
			Ephemeral: true,
		})
	}
	return result
}

func ephemeralMarker(sessionID string) string { return "ephemeral:" + sessionID }

func derivedConnections(nodes []ProjectedNode) []Connection {
	placed := map[string]ProjectedNode{}
	for _, node := range nodes {
		placed[node.ID] = node
	}
	connections := []Connection{}
	for _, node := range nodes {
		if node.State == NodeForbidden {
			continue
		}
		if node.Kind != EntitySession || node.Session == nil || node.Session.Record.PlanRef == nil {
			continue
		}
		source := ""
		kind := "session_launched_from"
		candidate := planNodeID(node.Session.Record.PlanRef.ItemID)
		if _, ok := placed[candidate]; ok {
			source = candidate
		}
		if source == "" || source == node.ID {
			continue
		}
		connections = append(connections, Connection{ID: fmt.Sprintf("%s:%s:%s", kind, source, node.ID), Source: source, Target: node.ID, Kind: kind, SourceOfTruth: "derived"})
	}
	sort.Slice(connections, func(i, j int) bool { return connections[i].ID < connections[j].ID })
	return connections
}

func workspaceNodeID(id string) string { return "workspace:" + id }

func planNodeID(id string) string { return "plan:" + id }

func sessionNodeID(id string) string { return "session:" + id }

func planRef(item models.ItemSummary) EntityRef {
	// No ObservedCommit: every item on a branch carries that branch's tip, so
	// recording it here marked each plan node stale on any commit to the repo.
	// Plan nodes render the current checkout, and staleness means the plan is gone.
	return EntityRef{Kind: EntityPlan, WorkspaceID: item.WorkspaceID, ItemID: item.ID, ItemPath: item.ItemPath, Identifier: item.Identifier, BranchKey: item.Branch}
}

func sessionRef(session ai.SessionRecordView) EntityRef {
	return EntityRef{Kind: EntitySession, WorkspaceID: session.WorkspaceID, BranchKey: session.RequestedBranch, ObservedCommit: session.ObservedCommit, SessionID: session.ID}
}
