package canvas

import "time"

type EntityKind string

const (
	EntityWorkspace EntityKind = "workspace"
	EntityPlan      EntityKind = "plan"
	EntitySession   EntityKind = "session"
)

type Viewport struct {
	X    float64 `json:"x" yaml:"x"`
	Y    float64 `json:"y" yaml:"y"`
	Zoom float64 `json:"zoom" yaml:"zoom"`
}

type Position struct {
	X float64 `json:"x" yaml:"x"`
	Y float64 `json:"y" yaml:"y"`
}

type EntityRef struct {
	Kind           EntityKind `json:"kind" yaml:"kind"`
	WorkspaceID    string     `json:"workspaceId" yaml:"workspaceId"`
	ItemID         string     `json:"itemId,omitempty" yaml:"itemId,omitempty"`
	ItemPath       string     `json:"itemPath,omitempty" yaml:"itemPath,omitempty"`
	Identifier     string     `json:"identifier,omitempty" yaml:"identifier,omitempty"`
	BranchKey      string     `json:"branchKey,omitempty" yaml:"branchKey,omitempty"`
	ObservedCommit string     `json:"observedCommit,omitempty" yaml:"observedCommit,omitempty"`
	SessionID      string     `json:"sessionId,omitempty" yaml:"sessionId,omitempty"`
}

type Layout struct {
	ID          string    `json:"id" yaml:"id"`
	OwnerUserID string    `json:"ownerUserId,omitempty" yaml:"ownerUserId,omitempty"`
	WorkspaceID string    `json:"workspaceId" yaml:"workspaceId"`
	BranchKey   string    `json:"branchKey" yaml:"branchKey"`
	Viewport    Viewport  `json:"viewport" yaml:"viewport"`
	Version     int64     `json:"version" yaml:"version"`
	CreatedAt   time.Time `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt" yaml:"updatedAt"`
}

type Placement struct {
	LayoutID  string    `json:"layoutId" yaml:"layoutId"`
	NodeID    string    `json:"nodeId" yaml:"nodeId"`
	EntityRef EntityRef `json:"entityRef" yaml:"entityRef"`
	Position  Position  `json:"position" yaml:"position"`
	Collapsed bool      `json:"collapsed" yaml:"collapsed"`
	Hidden    bool      `json:"hidden,omitempty" yaml:"hidden,omitempty"`
	Revision  int64     `json:"revision" yaml:"revision"`
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
}

type PlacementPatch struct {
	NodeID           string    `json:"nodeId"`
	EntityRef        EntityRef `json:"entityRef,omitempty"`
	Position         Position  `json:"position"`
	Collapsed        bool      `json:"collapsed"`
	ExpectedRevision int64     `json:"expectedRevision"`
}

type Snapshot struct {
	Layouts    []Layout    `json:"layouts" yaml:"layouts"`
	Placements []Placement `json:"placements" yaml:"placements"`
}
