package canvas

import (
	"errors"
	"fmt"
	"sort"
	"strings"

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

type sessionReader interface {
	SessionRecords(string, string) ([]ai.SessionRecordView, error)
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
	existed := false
	layouts, err := s.repository.Layouts()
	if err != nil {
		return Projection{}, err
	}
	for _, layout := range layouts {
		if layout.OwnerUserID == ownerUserID && layout.WorkspaceID == workspaceID && layout.BranchKey == branchKey {
			existed = true
			break
		}
	}
	layout, err := s.repository.ResolveDefault(ownerUserID, workspaceID, branchKey)
	if err != nil {
		return Projection{}, err
	}
	if !existed {
		if err := s.seedInitialPlacements(layout); err != nil {
			return Projection{}, err
		}
	}
	return s.Project(ownerUserID, layout.ID)
}

func (s *Service) Project(ownerUserID, layoutID string) (Projection, error) {
	layout, found, err := s.repository.GetLayout(layoutID)
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
	items, err := s.items.BranchItems(layout.WorkspaceID, layout.BranchKey)
	if err != nil {
		return Projection{}, err
	}
	items = canvasPlanItems(items)
	sessions := []ai.SessionRecordView{}
	if s.sessions != nil {
		sessions, err = s.sessions.SessionRecords(layout.WorkspaceID, layout.BranchKey)
		if err != nil {
			return Projection{}, err
		}
	}
	placements, err := s.repository.Placements(layout.ID)
	if err != nil {
		return Projection{}, err
	}
	return s.project(layout, workspaceConfig, items, sessions, placements), nil
}

func (s *Service) PatchPlacements(ownerUserID, layoutID string, patches []PlacementPatch) (Projection, error) {
	if _, err := s.ownedLayout(ownerUserID, layoutID); err != nil {
		return Projection{}, err
	}
	if _, err := s.repository.PatchPlacements(layoutID, patches); err != nil {
		return Projection{}, err
	}
	return s.Project(ownerUserID, layoutID)
}

func (s *Service) SaveViewport(ownerUserID, layoutID string, expectedVersion int64, viewport Viewport) (Projection, error) {
	if _, err := s.ownedLayout(ownerUserID, layoutID); err != nil {
		return Projection{}, err
	}
	if _, err := s.repository.SaveViewport(layoutID, expectedVersion, viewport); err != nil {
		return Projection{}, err
	}
	return s.Project(ownerUserID, layoutID)
}

func (s *Service) RemovePlacement(ownerUserID, layoutID, nodeID string, expectedRevision int64) (Projection, error) {
	if _, err := s.ownedLayout(ownerUserID, layoutID); err != nil {
		return Projection{}, err
	}
	if err := s.repository.RemovePlacement(layoutID, nodeID, expectedRevision); err != nil {
		return Projection{}, err
	}
	return s.Project(ownerUserID, layoutID)
}

func (s *Service) ownedLayout(ownerUserID, layoutID string) (Layout, error) {
	layout, found, err := s.repository.GetLayout(layoutID)
	if err != nil {
		return Layout{}, err
	}
	if !found || layout.OwnerUserID != ownerUserID {
		return Layout{}, ErrNotFound
	}
	return layout, nil
}

func (s *Service) seedInitialPlacements(layout Layout) error {
	items, err := s.items.BranchItems(layout.WorkspaceID, layout.BranchKey)
	if err != nil {
		return err
	}
	items = canvasPlanItems(items)
	sessions := []ai.SessionRecordView{}
	if s.sessions != nil {
		sessions, err = s.sessions.SessionRecords(layout.WorkspaceID, layout.BranchKey)
		if err != nil {
			return err
		}
	}
	patches := []PlacementPatch{{NodeID: workspaceNodeID(layout.WorkspaceID), EntityRef: EntityRef{Kind: EntityWorkspace, WorkspaceID: layout.WorkspaceID}, Position: Position{X: 0, Y: 0}}}
	for i, item := range items {
		if len(patches) == MaxPlacements {
			break
		}
		patches = append(patches, PlacementPatch{NodeID: planNodeID(item.ID), EntityRef: planRef(item), Position: Position{X: 360 + float64(i%3)*320, Y: float64(i/3) * 190}})
	}
	for i, session := range sessions {
		if len(patches) == MaxPlacements {
			break
		}
		patches = append(patches, PlacementPatch{NodeID: sessionNodeID(session.ID), EntityRef: sessionRef(session), Position: Position{X: 360 + float64(i%3)*320, Y: 420 + float64(i/3)*190}, Collapsed: true})
	}
	for start := 0; start < len(patches); start += MaxPlacementBatch {
		end := start + MaxPlacementBatch
		if end > len(patches) {
			end = len(patches)
		}
		if _, err := s.repository.PatchPlacements(layout.ID, patches[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) project(layout Layout, workspaceConfig models.WorkspaceConfig, items []models.ItemSummary, sessions []ai.SessionRecordView, placements []Placement) Projection {
	itemByID := map[string]models.ItemSummary{}
	for _, item := range items {
		itemByID[item.ID] = item
	}
	sessionByID := map[string]ai.SessionRecordView{}
	for _, session := range sessions {
		sessionByID[session.ID] = session
	}
	currentBranch, commit := "", ""
	var gitStatus *models.GitStatus
	contentAvailable := strings.TrimSpace(workspaceConfig.Path) != ""
	if s.git != nil && contentAvailable {
		currentBranch, _ = s.git.CurrentBranch(workspaceConfig.Path)
		_, commit, _ = s.git.ResolveBranch(workspaceConfig.Path, layout.BranchKey)
		if status, statusErr := s.git.Status(workspaceConfig.ID, workspaceConfig.Path); statusErr == nil {
			gitStatus = &status
		} else {
			contentAvailable = false
		}
	}
	authorization := s.authorization
	if authorization == nil {
		authorization = map[models.Capability]bool{models.CapabilityRead: true, models.CapabilityWrite: true, models.CapabilityGit: true, models.CapabilityTerminal: true, models.CapabilityVerification: true}
	}
	axes := workspace.ProviderAxes(s.mode, s.datastore, workspaceConfig)
	actions := workspace.ResolveActionCapabilities(workspace.ActionCapabilityInput{Axes: axes, Authorization: authorization, ContentAvailable: contentAvailable, ExecutionAvailable: axes.ExecutionProvider != models.WorkspaceExecutionProviderNone, Writable: workspaceConfig.AccessMode != models.WorkspaceAccessModeRemoteSnapshot, CurrentBranch: currentBranch, TargetBranch: layout.BranchKey})
	readForbidden := actions[models.WorkspaceActionRepositoryRead].State == models.ActionCapabilityForbidden
	placed := map[string]bool{}
	nodes := make([]ProjectedNode, 0, len(placements))
	for _, placement := range placements {
		placed[placement.NodeID] = true
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
				node.State = NodeStale
				break
			}
			if placement.EntityRef.ObservedCommit != "" && item.Commit != "" && placement.EntityRef.ObservedCommit != item.Commit {
				node.State = NodeStale
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

func canvasPlanServiceFromPath(itemPath string) string {
	parts := strings.Split(strings.Trim(strings.ReplaceAll(itemPath, `\`, "/"), "/"), "/")
	if len(parts) != 3 || parts[0] != "plans" || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		return ""
	}
	return parts[1]
}

func derivedConnections(nodes []ProjectedNode) []Connection {
	placed := map[string]ProjectedNode{}
	workspaceID := ""
	for _, node := range nodes {
		placed[node.ID] = node
		if node.Kind == EntityWorkspace && node.State != NodeForbidden {
			workspaceID = node.ID
		}
	}
	connections := []Connection{}
	for _, node := range nodes {
		if node.State == NodeForbidden {
			continue
		}
		source := workspaceID
		kind := "repository_contains"
		if node.Kind == EntitySession && node.Session != nil && node.Session.Record.PlanRef != nil {
			candidate := planNodeID(node.Session.Record.PlanRef.ItemID)
			if _, ok := placed[candidate]; ok {
				source, kind = candidate, "session_launched_from"
			}
		}
		if source == "" || source == node.ID || (node.Kind != EntityPlan && node.Kind != EntitySession) {
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
	return EntityRef{Kind: EntityPlan, WorkspaceID: item.WorkspaceID, ItemID: item.ID, ItemPath: item.ItemPath, Identifier: item.Identifier, BranchKey: item.Branch, ObservedCommit: item.Commit}
}

func sessionRef(session ai.SessionRecordView) EntityRef {
	return EntityRef{Kind: EntitySession, WorkspaceID: session.WorkspaceID, BranchKey: session.RequestedBranch, ObservedCommit: session.ObservedCommit, SessionID: session.ID}
}
