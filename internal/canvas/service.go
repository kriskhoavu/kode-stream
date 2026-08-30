package canvas

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"kode-stream/internal/ai"
	"kode-stream/internal/common/models"
	"kode-stream/internal/item/index"
	"kode-stream/internal/verification"
	"kode-stream/internal/workspace"
	"kode-stream/internal/workspace/registry"
)

type NodeState string

const (
	NodeResolved  NodeState = "resolved"
	NodeStale     NodeState = "stale"
	NodeForbidden NodeState = "forbidden"
)

const maxProjectionCandidates = MaxPlacements

type WorkspaceNode struct {
	ID           string                                             `json:"id"`
	Name         string                                             `json:"name,omitempty"`
	Branch       string                                             `json:"branch,omitempty"`
	Commit       string                                             `json:"commit,omitempty"`
	Git          *models.GitStatus                                  `json:"git,omitempty"`
	ProviderAxes models.WorkspaceProviderAxes                       `json:"providerAxes"`
	Actions      map[models.WorkspaceAction]models.ActionCapability `json:"actions"`
	Verification *verification.Job                                  `json:"verification,omitempty"`
}

type PlanNode struct {
	ItemID     string                                             `json:"itemId"`
	Identifier string                                             `json:"identifier,omitempty"`
	Title      string                                             `json:"title,omitempty"`
	Service    string                                             `json:"service,omitempty"`
	Status     models.ItemStatus                                  `json:"status"`
	Branch     string                                             `json:"branch,omitempty"`
	Commit     string                                             `json:"commit,omitempty"`
	Editable   bool                                               `json:"editable"`
	Actions    map[models.WorkspaceAction]models.ActionCapability `json:"actions"`
}

type SessionNode struct {
	Record ai.SessionRecordView `json:"record"`
}

type ProjectedNode struct {
	ID        string         `json:"id"`
	Kind      EntityKind     `json:"kind"`
	State     NodeState      `json:"state"`
	EntityRef EntityRef      `json:"entityRef"`
	Position  Position       `json:"position"`
	Collapsed bool           `json:"collapsed"`
	Revision  int64          `json:"revision"`
	Workspace *WorkspaceNode `json:"workspace,omitempty"`
	Plan      *PlanNode      `json:"plan,omitempty"`
	Session   *SessionNode   `json:"session,omitempty"`
}

type Connection struct {
	ID            string `json:"id"`
	Source        string `json:"source"`
	Target        string `json:"target"`
	Kind          string `json:"kind"`
	SourceOfTruth string `json:"sourceOfTruth"`
}

type Projection struct {
	Layout      Layout          `json:"layout"`
	Nodes       []ProjectedNode `json:"nodes"`
	Connections []Connection    `json:"connections"`
	Unplaced    []EntityRef     `json:"unplaced"`
}

type gitReader interface {
	CurrentBranch(string) (string, error)
	ResolveBranch(string, string) (string, string, error)
	Status(string, string) (models.GitStatus, error)
}
type contextGitReader interface {
	CurrentBranchContext(context.Context, string) (string, error)
	ResolveBranchContext(context.Context, string, string) (string, string, error)
	StatusContext(context.Context, string, string) (models.GitStatus, error)
}

type sessionReader interface {
	SessionRecords(string, string) ([]ai.SessionRecordView, error)
}
type contextSessionReader interface {
	SessionRecordsContext(context.Context, string, string) ([]ai.SessionRecordView, error)
}

type verificationReader interface {
	Latest(string) (verification.Job, bool)
}

type Service struct {
	repository    Repository
	workspaces    registry.Repository
	items         itemindex.Repository
	git           gitReader
	sessions      sessionReader
	verification  verificationReader
	mode          models.RuntimeMode
	datastore     models.AppStateDatastore
	authorization map[models.Capability]bool
}

func NewService(repository Repository, workspaces registry.Repository, items itemindex.Repository, git gitReader, sessions sessionReader, verification verificationReader, mode models.RuntimeMode, datastore models.AppStateDatastore, authorization map[models.Capability]bool) *Service {
	return &Service{repository: repository, workspaces: workspaces, items: items, git: git, sessions: sessions, verification: verification, mode: mode, datastore: datastore, authorization: authorization}
}

func (s *Service) ResolveDefault(ownerUserID, workspaceID, branchKey string) (Projection, error) {
	return s.ResolveDefaultContext(context.Background(), ownerUserID, workspaceID, branchKey)
}

func (s *Service) ResolveDefaultContext(ctx context.Context, ownerUserID, workspaceID, branchKey string) (Projection, error) {
	if err := ctx.Err(); err != nil {
		return Projection{}, err
	}
	if s.repository == nil {
		return Projection{}, errors.New("canvas repository is unavailable")
	}
	branchKey = strings.TrimSpace(branchKey)
	if branchKey == "" {
		workspaceConfig, found, err := s.workspaces.Get(workspaceID)
		if err != nil {
			return Projection{}, err
		}
		if !found {
			return Projection{}, ErrNotFound
		}
		if s.git != nil && strings.TrimSpace(workspaceConfig.Path) != "" {
			branchKey, _ = s.git.CurrentBranch(workspaceConfig.Path)
		}
		if branchKey == "" {
			branchKey = workspaceConfig.BaselineBranch
		}
	}
	if layout, found, err := s.repository.FindDefault(ownerUserID, workspaceID, branchKey); err != nil {
		return Projection{}, err
	} else if found {
		return s.ProjectContext(ctx, ownerUserID, layout.ID)
	}
	seed, err := s.initialPlacements(ctx, workspaceID, branchKey)
	if err != nil {
		return Projection{}, err
	}
	layout, _, err := s.repository.InitializeDefaultContext(ctx, ownerUserID, workspaceID, branchKey, seed)
	if err != nil {
		return Projection{}, err
	}
	return s.ProjectContext(ctx, ownerUserID, layout.ID)
}

func (s *Service) Project(ownerUserID, layoutID string) (Projection, error) {
	return s.ProjectContext(context.Background(), ownerUserID, layoutID)
}

func (s *Service) ProjectContext(ctx context.Context, ownerUserID, layoutID string) (Projection, error) {
	if err := ctx.Err(); err != nil {
		return Projection{}, err
	}
	layout, found, err := s.getLayoutContext(ctx, layoutID)
	if err != nil {
		return Projection{}, err
	}
	if !found || layout.OwnerUserID != ownerUserID {
		return Projection{}, ErrNotFound
	}
	workspaceConfig, found, err := s.workspaces.Get(layout.WorkspaceID)
	if err != nil {
		return Projection{}, err
	}
	if !found {
		return Projection{}, ErrNotFound
	}
	items, err := s.branchItemsContext(ctx, layout.WorkspaceID, layout.BranchKey)
	if err != nil {
		return Projection{}, err
	}
	items = boundedPlanItems(ctx, items)
	sessions := []ai.SessionRecordView{}
	if s.sessions != nil {
		sessions, err = s.sessionRecordsContext(ctx, layout.WorkspaceID, layout.BranchKey)
		if err != nil {
			return Projection{}, err
		}
	}
	if len(sessions) > maxProjectionCandidates {
		sessions = sessions[:maxProjectionCandidates]
	}
	if err := ctx.Err(); err != nil {
		return Projection{}, err
	}
	placements, err := s.placementsContext(ctx, layout.ID)
	if err != nil {
		return Projection{}, err
	}
	return s.projectContext(ctx, layout, workspaceConfig, items, sessions, placements), nil
}

func (s *Service) PatchPlacements(ownerUserID, layoutID string, patches []PlacementPatch) (Projection, error) {
	return s.PatchPlacementsContext(context.Background(), ownerUserID, layoutID, patches)
}

func (s *Service) PatchPlacementsContext(ctx context.Context, ownerUserID, layoutID string, patches []PlacementPatch) (Projection, error) {
	if err := ctx.Err(); err != nil {
		return Projection{}, err
	}
	layout, err := s.ownedLayoutContext(ctx, ownerUserID, layoutID)
	if err != nil {
		return Projection{}, err
	}
	patches, err = s.canonicalizeNewPlacementPatchesContext(ctx, layout, patches)
	if err != nil {
		return Projection{}, err
	}
	if _, err := s.patchPlacementsContext(ctx, layoutID, patches); err != nil {
		return Projection{}, err
	}
	return s.ProjectContext(ctx, ownerUserID, layoutID)
}

func (s *Service) canonicalizeNewPlacementPatches(layout Layout, patches []PlacementPatch) ([]PlacementPatch, error) {
	return s.canonicalizeNewPlacementPatchesContext(context.Background(), layout, patches)
}

func (s *Service) canonicalizeNewPlacementPatchesContext(ctx context.Context, layout Layout, patches []PlacementPatch) ([]PlacementPatch, error) {
	placements, err := s.placementsContext(ctx, layout.ID)
	if err != nil {
		return nil, err
	}
	existing := make(map[string]bool, len(placements))
	for _, placement := range placements {
		existing[placement.NodeID] = true
	}
	result := append([]PlacementPatch(nil), patches...)
	for i := range result {
		patch := &result[i]
		if existing[patch.NodeID] {
			continue
		}
		if patch.ExpectedRevision != 0 {
			continue // The repository reports the concurrency conflict.
		}
		switch patch.EntityRef.Kind {
		case EntityWorkspace:
			if patch.NodeID != workspaceNodeID(layout.WorkspaceID) || patch.EntityRef.WorkspaceID != layout.WorkspaceID {
				return nil, errors.New("canvas workspace placement identity is invalid")
			}
			patch.EntityRef = EntityRef{Kind: EntityWorkspace, WorkspaceID: layout.WorkspaceID}
		case EntityPlan:
			items, err := s.branchItemsContext(ctx, layout.WorkspaceID, layout.BranchKey)
			if err != nil {
				return nil, err
			}
			found := false
			for _, item := range canvasPlanItems(items) {
				if item.ID == patch.EntityRef.ItemID && patch.NodeID == planNodeID(item.ID) {
					patch.EntityRef = planRef(item)
					found = true
					break
				}
			}
			if !found {
				return nil, errors.New("canvas plan placement does not resolve on this branch")
			}
		case EntitySession:
			if s.sessions == nil {
				return nil, errors.New("canvas session placement does not resolve")
			}
			sessions, err := s.sessionRecordsContext(ctx, layout.WorkspaceID, layout.BranchKey)
			if err != nil {
				return nil, err
			}
			found := false
			for _, session := range sessions {
				if session.ID == patch.EntityRef.SessionID && patch.NodeID == sessionNodeID(session.ID) {
					patch.EntityRef = sessionRef(session)
					found = true
					break
				}
			}
			if !found {
				return nil, errors.New("canvas session placement does not resolve on this branch")
			}
		default:
			return nil, errors.New("canvas placement entity kind is invalid")
		}
	}
	return result, nil
}

func (s *Service) SaveViewport(ownerUserID, layoutID string, expectedVersion int64, viewport Viewport) (Projection, error) {
	return s.SaveViewportContext(context.Background(), ownerUserID, layoutID, expectedVersion, viewport)
}
func (s *Service) SaveViewportContext(ctx context.Context, ownerUserID, layoutID string, expectedVersion int64, viewport Viewport) (Projection, error) {
	if err := ctx.Err(); err != nil {
		return Projection{}, err
	}
	if _, err := s.ownedLayoutContext(ctx, ownerUserID, layoutID); err != nil {
		return Projection{}, err
	}
	if _, err := s.saveViewportContext(ctx, layoutID, expectedVersion, viewport); err != nil {
		return Projection{}, err
	}
	return s.ProjectContext(ctx, ownerUserID, layoutID)
}

func (s *Service) RemovePlacement(ownerUserID, layoutID, nodeID string, expectedRevision int64) (Projection, error) {
	return s.RemovePlacementContext(context.Background(), ownerUserID, layoutID, nodeID, expectedRevision)
}
func (s *Service) RemovePlacementContext(ctx context.Context, ownerUserID, layoutID, nodeID string, expectedRevision int64) (Projection, error) {
	if err := ctx.Err(); err != nil {
		return Projection{}, err
	}
	if _, err := s.ownedLayoutContext(ctx, ownerUserID, layoutID); err != nil {
		return Projection{}, err
	}
	if err := s.removePlacementContext(ctx, layoutID, nodeID, expectedRevision); err != nil {
		return Projection{}, err
	}
	return s.ProjectContext(ctx, ownerUserID, layoutID)
}

func (s *Service) ownedLayout(ownerUserID, layoutID string) (Layout, error) {
	return s.ownedLayoutContext(context.Background(), ownerUserID, layoutID)
}

func (s *Service) ownedLayoutContext(ctx context.Context, ownerUserID, layoutID string) (Layout, error) {
	layout, found, err := s.getLayoutContext(ctx, layoutID)
	if err != nil {
		return Layout{}, err
	}
	if !found || layout.OwnerUserID != ownerUserID {
		return Layout{}, ErrNotFound
	}
	return layout, nil
}

func (s *Service) getLayoutContext(ctx context.Context, id string) (Layout, bool, error) {
	if repository, ok := s.repository.(ContextRepository); ok {
		return repository.GetLayoutContext(ctx, id)
	}
	if err := ctx.Err(); err != nil {
		return Layout{}, false, err
	}
	return s.repository.GetLayout(id)
}
func (s *Service) placementsContext(ctx context.Context, id string) ([]Placement, error) {
	if repository, ok := s.repository.(ContextRepository); ok {
		return repository.PlacementsContext(ctx, id)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.repository.Placements(id)
}
func (s *Service) patchPlacementsContext(ctx context.Context, id string, patches []PlacementPatch) ([]Placement, error) {
	if repository, ok := s.repository.(ContextRepository); ok {
		return repository.PatchPlacementsContext(ctx, id, patches)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.repository.PatchPlacements(id, patches)
}
func (s *Service) removePlacementContext(ctx context.Context, id, node string, revision int64) error {
	if repository, ok := s.repository.(ContextRepository); ok {
		return repository.RemovePlacementContext(ctx, id, node, revision)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.repository.RemovePlacement(id, node, revision)
}
func (s *Service) saveViewportContext(ctx context.Context, id string, version int64, viewport Viewport) (Layout, error) {
	if repository, ok := s.repository.(ContextRepository); ok {
		return repository.SaveViewportContext(ctx, id, version, viewport)
	}
	if err := ctx.Err(); err != nil {
		return Layout{}, err
	}
	return s.repository.SaveViewport(id, version, viewport)
}

func (s *Service) initialPlacements(ctx context.Context, workspaceID, branchKey string) ([]Placement, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	items, err := s.branchItemsContext(ctx, workspaceID, branchKey)
	if err != nil {
		return nil, err
	}
	items = boundedPlanItems(ctx, items)
	sessions := []ai.SessionRecordView{}
	if s.sessions != nil {
		sessions, err = s.sessionRecordsContext(ctx, workspaceID, branchKey)
		if err != nil {
			return nil, err
		}
	}
	if len(sessions) > maxProjectionCandidates {
		sessions = sessions[:maxProjectionCandidates]
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	placements := []Placement{{NodeID: workspaceNodeID(workspaceID), EntityRef: EntityRef{Kind: EntityWorkspace, WorkspaceID: workspaceID}, Position: Position{X: 0, Y: 0}, Revision: 1, UpdatedAt: now}}
	for i, item := range items {
		if len(placements) == MaxPlacements {
			break
		}
		placements = append(placements, Placement{NodeID: planNodeID(item.ID), EntityRef: planRef(item), Position: Position{X: 360 + float64(i%3)*320, Y: float64(i/3) * 190}, Revision: 1, UpdatedAt: now})
	}
	for i, session := range sessions {
		if len(placements) == MaxPlacements {
			break
		}
		placements = append(placements, Placement{NodeID: sessionNodeID(session.ID), EntityRef: sessionRef(session), Position: Position{X: 360 + float64(i%3)*320, Y: 420 + float64(i/3)*190}, Collapsed: true, Revision: 1, UpdatedAt: now})
	}
	return placements, nil
}

func (s *Service) branchItemsContext(ctx context.Context, workspaceID, branch string) ([]models.ItemSummary, error) {
	items := make([]models.ItemSummary, 0, maxProjectionCandidates)
	err := s.items.VisitContext(ctx, itemindex.Query{WorkspaceID: workspaceID, Branch: branch, IncludeSnapshots: true}, func(item models.ItemSummary) bool {
		if canvasPlanService(item) == "" {
			return true
		}
		items = append(items, item)
		return len(items) < maxProjectionCandidates
	})
	return items, err
}
func (s *Service) sessionRecordsContext(ctx context.Context, workspaceID, branch string) ([]ai.SessionRecordView, error) {
	if reader, ok := s.sessions.(contextSessionReader); ok {
		return reader.SessionRecordsContext(ctx, workspaceID, branch)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.sessions.SessionRecords(workspaceID, branch)
}

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

func (s *Service) projectContext(ctx context.Context, layout Layout, workspaceConfig models.WorkspaceConfig, items []models.ItemSummary, sessions []ai.SessionRecordView, placements []Placement) Projection {
	itemByID := map[string]models.ItemSummary{}
	// Placements reference items by ID, but the ID scheme has changed before
	// (see stablePlanID) without migrating stored placements. A path index lets
	// an orphaned placement rebind to the item it always meant, because
	// (workspace, branch, path) determines the current ID uniquely.
	itemByPath := map[string]models.ItemSummary{}
	for _, item := range items {
		itemByID[item.ID] = item
		itemByPath[canvasPathKey(item.ItemPath)] = item
	}
	sessionByID := map[string]ai.SessionRecordView{}
	for _, session := range sessions {
		sessionByID[session.ID] = session
	}
	currentBranch, commit := "", ""
	var gitStatus *models.GitStatus
	contentAvailable := strings.TrimSpace(workspaceConfig.Path) != ""
	if s.git != nil && contentAvailable {
		if contextual, ok := s.git.(contextGitReader); ok {
			currentBranch, _ = contextual.CurrentBranchContext(ctx, workspaceConfig.Path)
			_, commit, _ = contextual.ResolveBranchContext(ctx, workspaceConfig.Path, layout.BranchKey)
			if status, statusErr := contextual.StatusContext(ctx, workspaceConfig.ID, workspaceConfig.Path); statusErr == nil {
				gitStatus = &status
			} else {
				contentAvailable = false
			}
		} else if err := ctx.Err(); err != nil {
			contentAvailable = false
		} else {
			currentBranch, _ = s.git.CurrentBranch(workspaceConfig.Path)
			_, commit, _ = s.git.ResolveBranch(workspaceConfig.Path, layout.BranchKey)
			if status, statusErr := s.git.Status(workspaceConfig.ID, workspaceConfig.Path); statusErr == nil {
				gitStatus = &status
			} else {
				contentAvailable = false
			}
		}
	}
	authorization := s.authorization
	if authorization == nil {
		authorization = map[models.Capability]bool{models.CapabilityRead: true, models.CapabilityWrite: true, models.CapabilityGit: true, models.CapabilityTerminal: true, models.CapabilityVerification: true}
	}
	axes := workspace.ProviderAxes(s.mode, s.datastore, workspaceConfig)
	actions := workspace.ResolveActionCapabilities(workspace.ActionCapabilityInput{Axes: axes, Authorization: authorization, ContentAvailable: contentAvailable, ExecutionAvailable: axes.ExecutionProvider != models.WorkspaceExecutionProviderNone, Writable: workspaceConfig.AccessMode != models.WorkspaceAccessModeRemoteSnapshot, CurrentBranch: currentBranch, TargetBranch: layout.BranchKey})
	readForbidden := actions[models.WorkspaceActionRepositoryRead].State == models.ActionCapabilityForbidden
	// Filled before projecting so a rebind can tell whether the item is already
	// placed under its current ID, rather than depending on placement order.
	placed := map[string]bool{}
	for _, placement := range placements {
		placed[placement.NodeID] = true
	}
	nodes := make([]ProjectedNode, 0, len(placements))
	for _, placement := range placements {
		if placement.Hidden {
			continue
		}
		node := ProjectedNode{ID: placement.NodeID, Kind: placement.EntityRef.Kind, State: NodeResolved, EntityRef: placement.EntityRef, Position: placement.Position, Collapsed: placement.Collapsed, Revision: placement.Revision}
		if readForbidden {
			node.State = NodeForbidden
			node.EntityRef.Identifier = ""
			node.EntityRef.ItemPath = ""
			nodes = append(nodes, node)
			continue
		}
		switch placement.EntityRef.Kind {
		case EntityWorkspace:
			view := WorkspaceNode{ID: workspaceConfig.ID, Name: workspaceConfig.Name, Branch: currentBranch, Commit: commit, Git: gitStatus, ProviderAxes: axes, Actions: actions}
			if s.verification != nil {
				if latest, ok := s.verification.Latest(workspaceConfig.ID); ok {
					view.Verification = &latest
				}
			}
			node.Workspace = &view
		case EntityPlan:
			item, ok := itemByID[placement.EntityRef.ItemID]
			if !ok {
				// Canvas only projects ticket roots under plans/{service}/{ticket}.
				// Ignore placements left behind by older versions that treated every
				// indexed item (including wiki folders) as a plan.
				if canvasPlanServiceFromPath(placement.EntityRef.ItemPath) == "" {
					continue
				}
				rebound, found := itemByPath[canvasPathKey(placement.EntityRef.ItemPath)]
				if !found {
					// The plan itself is gone. Keep the placement stale so it stays
					// recoverable instead of being deleted behind the user's back.
					node.State = NodeStale
					break
				}
				if placed[planNodeID(rebound.ID)] {
					// The item is already placed under its current ID, so this
					// orphan is a superseded duplicate of that node.
					continue
				}
				// Adopt the placement: re-anchor it to the current identity so the
				// node resolves under the ID scheme in use now rather than the one
				// it was placed under.
				placed[planNodeID(rebound.ID)] = true
				item = rebound
				node.EntityRef.ItemID = item.ID
			}
			node.Plan = &PlanNode{ItemID: item.ID, Identifier: item.Identifier, Title: item.Title, Service: canvasPlanService(item), Status: item.Status, Branch: item.Branch, Commit: item.Commit, Editable: item.Editable, Actions: actions}
		case EntitySession:
			session, ok := sessionByID[placement.EntityRef.SessionID]
			if !ok {
				node.State = NodeStale
				break
			}
			node.Session = &SessionNode{Record: session}
		default:
			node.State = NodeStale
		}
		nodes = append(nodes, node)
	}
	unplaced := []EntityRef{}
	if !placed[workspaceNodeID(layout.WorkspaceID)] {
		unplaced = append(unplaced, EntityRef{Kind: EntityWorkspace, WorkspaceID: layout.WorkspaceID})
	}
	for _, item := range items {
		if !placed[planNodeID(item.ID)] {
			unplaced = append(unplaced, planRef(item))
		}
	}
	for _, session := range sessions {
		if !placed[sessionNodeID(session.ID)] {
			unplaced = append(unplaced, sessionRef(session))
		}
	}
	connections := derivedConnections(nodes)
	return Projection{Layout: layout, Nodes: nodes, Connections: connections, Unplaced: unplaced}
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
func planNodeID(id string) string      { return "plan:" + id }
func sessionNodeID(id string) string   { return "session:" + id }

func planRef(item models.ItemSummary) EntityRef {
	// No ObservedCommit: every item on a branch carries that branch's tip, so
	// recording it here marked each plan node stale on any commit to the repo.
	// Plan nodes render the current checkout, and staleness means the plan is gone.
	return EntityRef{Kind: EntityPlan, WorkspaceID: item.WorkspaceID, ItemID: item.ID, ItemPath: item.ItemPath, Identifier: item.Identifier, BranchKey: item.Branch}
}

func sessionRef(session ai.SessionRecordView) EntityRef {
	return EntityRef{Kind: EntitySession, WorkspaceID: session.WorkspaceID, BranchKey: session.RequestedBranch, ObservedCommit: session.ObservedCommit, SessionID: session.ID}
}
