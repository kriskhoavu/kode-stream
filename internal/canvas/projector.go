package canvas

import (
	"context"
	"strings"

	"kode-stream/internal/ai"
	"kode-stream/internal/common/models"
	"kode-stream/internal/workspace"
)

type projector struct {
	s               *Service
	ctx             context.Context
	layout          Layout
	workspaceConfig models.WorkspaceConfig

	itemByID    map[string]models.ItemSummary
	itemByPath  map[string]models.ItemSummary
	sessionByID map[string]ai.SessionRecordView
	placed      map[string]bool

	currentBranch    string
	commit           string
	gitStatus        *models.GitStatus
	contentAvailable bool
	axes             models.WorkspaceProviderAxes
	actions          map[models.WorkspaceAction]models.ActionCapability
	readForbidden    bool
}

func (s *Service) projectContext(ctx context.Context, layout Layout, workspaceConfig models.WorkspaceConfig, items []models.ItemSummary, sessions []ai.SessionRecordView, placements []Placement) Projection {
	p := &projector{s: s, ctx: ctx, layout: layout, workspaceConfig: workspaceConfig}
	p.index(items, sessions, placements)
	p.readGit()
	p.resolveActions()
	nodes := make([]ProjectedNode, 0, len(placements))
	for _, placement := range placements {
		if node, keep := p.project(placement); keep {
			nodes = append(nodes, node)
		}
	}
	if !p.readForbidden {
		nodes = append(nodes, ephemeralSessionNodes(nodes, sessions, p.placed)...)
	}
	return Projection{Layout: layout, Nodes: nodes, Connections: derivedConnections(nodes), Unplaced: p.unplaced(items, sessions)}
}

// Rebind by path: (workspace, branch, path) determines the current ID after an ID scheme change.
func (p *projector) index(items []models.ItemSummary, sessions []ai.SessionRecordView, placements []Placement) {
	p.itemByID, p.itemByPath = map[string]models.ItemSummary{}, map[string]models.ItemSummary{}
	for _, item := range items {
		p.itemByID[item.ID] = item
		p.itemByPath[canvasPathKey(item.ItemPath)] = item
	}
	p.sessionByID = map[string]ai.SessionRecordView{}
	for _, session := range sessions {
		p.sessionByID[session.ID] = session
	}
	p.placed = map[string]bool{}
	for _, placement := range placements {
		p.placed[placement.NodeID] = true
	}
}

func (p *projector) readGit() {
	p.contentAvailable = strings.TrimSpace(p.workspaceConfig.Path) != ""
	if p.s.git == nil || !p.contentAvailable {
		return
	}
	path, id := p.workspaceConfig.Path, p.workspaceConfig.ID
	var status models.GitStatus
	var err error
	if contextual, ok := p.s.git.(contextGitReader); ok {
		p.currentBranch, _ = contextual.CurrentBranchContext(p.ctx, path)
		_, p.commit, _ = contextual.ResolveBranchContext(p.ctx, path, p.layout.BranchKey)
		status, err = contextual.StatusContext(p.ctx, id, path)
	} else if err = p.ctx.Err(); err == nil {
		p.currentBranch, _ = p.s.git.CurrentBranch(path)
		_, p.commit, _ = p.s.git.ResolveBranch(path, p.layout.BranchKey)
		status, err = p.s.git.Status(id, path)
	}
	if err != nil {
		p.contentAvailable = false
		return
	}
	p.gitStatus = &status
}

func (p *projector) resolveActions() {
	authorization := p.s.authorization
	if authorization == nil {
		authorization = map[models.Capability]bool{models.CapabilityRead: true, models.CapabilityWrite: true, models.CapabilityGit: true, models.CapabilityTerminal: true, models.CapabilityVerification: true}
	}
	p.axes = workspace.ProviderAxes(p.s.mode, p.s.datastore, p.workspaceConfig)
	p.actions = workspace.ResolveActionCapabilities(workspace.ActionCapabilityInput{Axes: p.axes, Authorization: authorization, ContentAvailable: p.contentAvailable, ExecutionAvailable: p.axes.ExecutionProvider != models.WorkspaceExecutionProviderNone, Writable: p.workspaceConfig.AccessMode != models.WorkspaceAccessModeRemoteSnapshot, CurrentBranch: p.currentBranch, TargetBranch: p.layout.BranchKey})
	p.readForbidden = p.actions[models.WorkspaceActionRepositoryRead].State == models.ActionCapabilityForbidden
}

func (p *projector) project(placement Placement) (ProjectedNode, bool) {
	if placement.Hidden {
		return ProjectedNode{}, false
	}
	node := ProjectedNode{ID: placement.NodeID, Kind: placement.EntityRef.Kind, State: NodeResolved, EntityRef: placement.EntityRef, Position: placement.Position, Size: placement.Size, Collapsed: placement.Collapsed, Revision: placement.Revision}
	if p.readForbidden {
		node.State = NodeForbidden
		node.EntityRef.Identifier = ""
		node.EntityRef.ItemPath = ""
		return node, true
	}
	switch placement.EntityRef.Kind {
	case EntityWorkspace:
		node.Workspace = p.workspaceNode()
	case EntityPlan:
		return p.planNode(node)
	case EntitySession:
		session, ok := p.sessionByID[placement.EntityRef.SessionID]
		if !ok {
			node.State = NodeStale
			break
		}
		node.Session = &SessionNode{Record: session}
	default:
		node.State = NodeStale
	}
	return node, true
}

func (p *projector) workspaceNode() *WorkspaceNode {
	view := WorkspaceNode{ID: p.workspaceConfig.ID, Name: p.workspaceConfig.Name, Branch: p.currentBranch, Commit: p.commit, Git: p.gitStatus, ProviderAxes: p.axes, Actions: p.actions}
	if p.s.verification != nil {
		if latest, ok := p.s.verification.Latest(p.workspaceConfig.ID); ok {
			view.Verification = &latest
		}
	}
	return &view
}

func (p *projector) planNode(node ProjectedNode) (ProjectedNode, bool) {
	item, ok := p.itemByID[node.EntityRef.ItemID]
	if !ok {
		// Placements from older versions may point at wiki folders. Only plans/{service}/{ticket} roots project.
		if canvasPlanServiceFromPath(node.EntityRef.ItemPath) == "" {
			return node, false
		}
		rebound, found := p.itemByPath[canvasPathKey(node.EntityRef.ItemPath)]
		if !found {
			// The plan is gone. Keep the placement stale, so it stays recoverable.
			node.State = NodeStale
			return node, true
		}
		if p.placed[planNodeID(rebound.ID)] {
			// The item is already placed under its current ID, so this
			// orphan is a superseded duplicate of that node.
			return node, false
		}
		// Re-anchor the placement to the item's current ID.
		p.placed[planNodeID(rebound.ID)] = true
		item = rebound
		node.EntityRef.ItemID = item.ID
	}
	node.Plan = &PlanNode{ItemID: item.ID, Identifier: item.Identifier, Title: item.Title, Service: canvasPlanService(item), Status: item.Status, Branch: item.Branch, Commit: item.Commit, Editable: item.Editable, Actions: p.actions}
	return node, true
}

func (p *projector) unplaced(items []models.ItemSummary, sessions []ai.SessionRecordView) []EntityRef {
	unplaced := []EntityRef{}
	if !p.placed[workspaceNodeID(p.layout.WorkspaceID)] {
		unplaced = append(unplaced, EntityRef{Kind: EntityWorkspace, WorkspaceID: p.layout.WorkspaceID})
	}
	for _, item := range items {
		if !p.placed[planNodeID(item.ID)] {
			unplaced = append(unplaced, planRef(item))
		}
	}
	for _, session := range sessions {
		if !p.placed[sessionNodeID(session.ID)] && !p.placed[ephemeralMarker(session.ID)] {
			unplaced = append(unplaced, sessionRef(session))
		}
	}
	return unplaced
}
