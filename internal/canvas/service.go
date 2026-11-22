package canvas

import (
	"context"
	"errors"
	"strings"
	"time"

	"kode-stream/internal/ai"
	"kode-stream/internal/common/models"
	"kode-stream/internal/item/index"
	"kode-stream/internal/verification"
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
	Size      *Size          `json:"size,omitempty"`
	Collapsed bool           `json:"collapsed"`
	Revision  int64          `json:"revision"`
	Workspace *WorkspaceNode `json:"workspace,omitempty"`
	Plan      *PlanNode      `json:"plan,omitempty"`
	Session   *SessionNode   `json:"session,omitempty"`
	// Ephemeral marks a live session drawn beside its plan without a placement.
	// It leaves the Canvas when the session stops unless it is placed.
	Ephemeral bool `json:"ephemeral,omitempty"`
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

// Ephemeral session nodes sit in a row to the right of their plan, sized like
// an expanded session node so the terminal shows; the constants mirror the
// Canvas board's focus layout.
const (
	ephemeralGutter        = 40
	ephemeralPlanWidth     = 245
	ephemeralSessionWidth  = 660
	ephemeralSessionHeight = 500
)
