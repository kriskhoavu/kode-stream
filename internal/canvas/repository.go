package canvas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	MaxPlacements          = 300
	MaxPlacementBatch      = 50
	MaxCoordinate          = 1_000_000
	MinZoom                = 0.1
	MaxZoom                = 2.0
	CurrentSnapshotVersion = 2
)

var ErrNotFound = errors.New("canvas layout not found")

type PlacementConflictError struct {
	NodeIDs []string
}

func (e *PlacementConflictError) Error() string {
	return fmt.Sprintf("canvas placement conflict: %s", strings.Join(e.NodeIDs, ", "))
}

type Repository interface {
	ResolveDefault(ownerUserID, workspaceID, branchKey string) (Layout, bool, error)
	FindDefault(ownerUserID, workspaceID, branchKey string) (Layout, bool, error)
	// InitializeDefault publishes a newly-created layout and its complete initial
	// projection together.  Callers must build the bounded deterministic seed
	// before this call; a failed publication must not leave a visible layout.
	InitializeDefault(ownerUserID, workspaceID, branchKey string, placements []Placement) (Layout, bool, error)
	InitializeDefaultContext(context.Context, string, string, string, []Placement) (Layout, bool, error)
	GetLayout(id string) (Layout, bool, error)
	Layouts() ([]Layout, error)
	Placements(layoutID string) ([]Placement, error)
	PatchPlacements(layoutID string, patches []PlacementPatch) ([]Placement, error)
	RemovePlacement(layoutID, nodeID string, expectedRevision int64) error
	SaveViewport(layoutID string, expectedVersion int64, viewport Viewport) (Layout, error)
	Snapshot() (Snapshot, error)
	ReplaceAll(Snapshot) error
}

// ContextRepository is the request-bound Canvas persistence seam. Production
// adapters implement it so HTTP cancellation reaches database/file work rather
// than being observed only after an unbounded operation returns.
type ContextRepository interface {
	GetLayoutContext(context.Context, string) (Layout, bool, error)
	PlacementsContext(context.Context, string) ([]Placement, error)
	PatchPlacementsContext(context.Context, string, []PlacementPatch) ([]Placement, error)
	RemovePlacementContext(context.Context, string, string, int64) error
	SaveViewportContext(context.Context, string, int64, Viewport) (Layout, error)
}

func NewLayout(ownerUserID, workspaceID, branchKey string, now time.Time) (Layout, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	workspaceID = strings.TrimSpace(workspaceID)
	branchKey = strings.TrimSpace(branchKey)
	if workspaceID == "" {
		return Layout{}, errors.New("canvas workspace ID is required")
	}
	if branchKey == "" {
		return Layout{}, errors.New("canvas branch key is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	key := ownerUserID + "\x00" + workspaceID + "\x00" + branchKey
	digest := sha256.Sum256([]byte(key))
	return Layout{
		ID:          "canvas-" + hex.EncodeToString(digest[:8]),
		OwnerUserID: ownerUserID,
		WorkspaceID: workspaceID,
		BranchKey:   branchKey,
		Viewport:    Viewport{Zoom: 1},
		Version:     1,
		CreatedAt:   now.UTC(),
		UpdatedAt:   now.UTC(),
	}, nil
}

func ValidateSnapshot(snapshot Snapshot) error {
	if snapshot.Layouts == nil {
		snapshot.Layouts = []Layout{}
	}
	if snapshot.Placements == nil {
		snapshot.Placements = []Placement{}
	}
	layouts := map[string]Layout{}
	unique := map[string]bool{}
	for _, layout := range snapshot.Layouts {
		if err := validateLayout(layout); err != nil {
			return err
		}
		if _, exists := layouts[layout.ID]; exists {
			return fmt.Errorf("duplicate canvas layout ID %q", layout.ID)
		}
		key := layout.OwnerUserID + "\x00" + layout.WorkspaceID + "\x00" + layout.BranchKey
		if unique[key] {
			return errors.New("duplicate default canvas layout")
		}
		unique[key] = true
		layouts[layout.ID] = layout
	}
	counts := map[string]int{}
	nodes := map[string]bool{}
	for _, placement := range snapshot.Placements {
		layout, exists := layouts[placement.LayoutID]
		if !exists {
			return fmt.Errorf("canvas placement %q has an unknown layout", placement.NodeID)
		}
		if err := validatePlacement(layout, placement); err != nil {
			return err
		}
		key := placement.LayoutID + "\x00" + placement.NodeID
		if nodes[key] {
			return fmt.Errorf("duplicate canvas placement %q", placement.NodeID)
		}
		nodes[key] = true
		counts[placement.LayoutID]++
		if counts[placement.LayoutID] > MaxPlacements {
			return fmt.Errorf("canvas layout exceeds %d placements", MaxPlacements)
		}
	}
	return nil
}

func validateLayout(layout Layout) error {
	if strings.TrimSpace(layout.ID) == "" || strings.TrimSpace(layout.WorkspaceID) == "" || strings.TrimSpace(layout.BranchKey) == "" {
		return errors.New("canvas layout identity is incomplete")
	}
	if layout.Version < 1 {
		return errors.New("canvas layout version must be positive")
	}
	return validateViewport(layout.Viewport)
}

func validatePlacement(layout Layout, placement Placement) error {
	if strings.TrimSpace(placement.NodeID) == "" {
		return errors.New("canvas node ID is required")
	}
	if placement.Revision < 1 {
		return errors.New("canvas placement revision must be positive")
	}
	if !finiteCoordinate(placement.Position.X) || !finiteCoordinate(placement.Position.Y) {
		return errors.New("canvas position is outside allowed limits")
	}
	return validateEntityRef(layout, placement.EntityRef)
}

func validateEntityRef(layout Layout, ref EntityRef) error {
	if strings.TrimSpace(ref.WorkspaceID) == "" || ref.WorkspaceID != layout.WorkspaceID {
		return errors.New("canvas entity reference must belong to its layout workspace")
	}
	switch ref.Kind {
	case EntityWorkspace:
		if ref.ItemID != "" || ref.SessionID != "" {
			return errors.New("workspace reference contains another entity identity")
		}
	case EntityPlan:
		if strings.TrimSpace(ref.ItemID) == "" || strings.TrimSpace(ref.ItemPath) == "" || strings.TrimSpace(ref.BranchKey) == "" {
			return errors.New("plan reference identity is incomplete")
		}
		if ref.BranchKey != layout.BranchKey {
			return errors.New("plan reference branch differs from its layout")
		}
	case EntitySession:
		if strings.TrimSpace(ref.SessionID) == "" {
			return errors.New("session reference ID is required")
		}
		if ref.BranchKey != "" && ref.BranchKey != layout.BranchKey {
			return errors.New("session reference branch differs from its layout")
		}
	default:
		return fmt.Errorf("unsupported canvas entity kind %q", ref.Kind)
	}
	return nil
}

func validateViewport(viewport Viewport) error {
	if !finiteCoordinate(viewport.X) || !finiteCoordinate(viewport.Y) || math.IsNaN(viewport.Zoom) || math.IsInf(viewport.Zoom, 0) || viewport.Zoom < MinZoom || viewport.Zoom > MaxZoom {
		return errors.New("canvas viewport is outside allowed limits")
	}
	return nil
}

func finiteCoordinate(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && math.Abs(value) <= MaxCoordinate
}

func normalizeSnapshot(snapshot Snapshot) Snapshot {
	if snapshot.Version < CurrentSnapshotVersion {
		for index := range snapshot.Placements {
			if snapshot.Placements[index].EntityRef.Kind == EntitySession {
				snapshot.Placements[index].Collapsed = true
			}
		}
		snapshot.Version = CurrentSnapshotVersion
	}
	if snapshot.Layouts == nil {
		snapshot.Layouts = []Layout{}
	}
	if snapshot.Placements == nil {
		snapshot.Placements = []Placement{}
	}
	return snapshot
}
