package canvas

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestFileRepositoryResolvesAndPatchesIndependentPlacements(t *testing.T) {
	repository := NewFileRepository(filepath.Join(t.TempDir(), "canvases.yaml"))
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	repository.now = func() time.Time { return now }
	layout, err := repository.ResolveDefault("", "workspace-1", "main")
	if err != nil {
		t.Fatal(err)
	}
	again, err := repository.ResolveDefault("", "workspace-1", "main")
	if err != nil || again.ID != layout.ID {
		t.Fatalf("resolved layout = %#v err=%v", again, err)
	}
	created, err := repository.PatchPlacements(layout.ID, []PlacementPatch{
		{NodeID: "workspace", EntityRef: EntityRef{Kind: EntityWorkspace, WorkspaceID: "workspace-1"}, Position: Position{X: 10, Y: 20}},
		{NodeID: "plan", EntityRef: EntityRef{Kind: EntityPlan, WorkspaceID: "workspace-1", ItemID: "item-1", ItemPath: "plans/PM-037", BranchKey: "main"}, Position: Position{X: 30, Y: 40}},
	})
	if err != nil || len(created) != 2 || created[0].Revision != 1 {
		t.Fatalf("created = %#v err=%v", created, err)
	}
	updated, err := repository.PatchPlacements(layout.ID, []PlacementPatch{{NodeID: "workspace", Position: Position{X: 50, Y: 60}, ExpectedRevision: 1}})
	if err != nil || len(updated) != 1 || updated[0].Revision != 2 {
		t.Fatalf("updated = %#v err=%v", updated, err)
	}
	placements, err := repository.Placements(layout.ID)
	if err != nil || len(placements) != 2 || placements[1].Position.X != 50 || placements[0].Position.X != 30 {
		t.Fatalf("placements = %#v err=%v", placements, err)
	}
	if err := repository.RemovePlacement(layout.ID, "plan", 1); err != nil {
		t.Fatal(err)
	}
	placements, err = repository.Placements(layout.ID)
	if err != nil || len(placements) != 2 || !placements[0].Hidden || placements[0].Revision != 2 {
		t.Fatalf("hidden placements = %#v err=%v", placements, err)
	}
}

func TestFileRepositoryMigratesLegacySessionsToCollapsedOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "canvases.yaml")
	layout, err := NewLayout("", "workspace-1", "main", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	legacy := Snapshot{Layouts: []Layout{layout}, Placements: []Placement{{LayoutID: layout.ID, NodeID: "session:one", EntityRef: EntityRef{Kind: EntitySession, WorkspaceID: "workspace-1", SessionID: "one", BranchKey: "main"}, Revision: 1, UpdatedAt: time.Now().UTC()}}}
	data, err := yaml.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	repository := NewFileRepository(path)
	placements, err := repository.Placements(layout.ID)
	if err != nil || len(placements) != 1 || !placements[0].Collapsed {
		t.Fatalf("migrated placements = %#v err=%v", placements, err)
	}
	updated, err := repository.PatchPlacements(layout.ID, []PlacementPatch{{NodeID: "session:one", Position: placements[0].Position, Collapsed: false, ExpectedRevision: placements[0].Revision}})
	if err != nil || len(updated) != 1 || updated[0].Collapsed {
		t.Fatalf("updated placements = %#v err=%v", updated, err)
	}
	snapshot, err := repository.Snapshot()
	if err != nil || snapshot.Version != CurrentSnapshotVersion || snapshot.Placements[0].Collapsed {
		t.Fatalf("snapshot = %#v err=%v", snapshot, err)
	}
}

func TestFileRepositoryRejectsConflictsAndInvalidReferences(t *testing.T) {
	repository := NewFileRepository(filepath.Join(t.TempDir(), "canvases.yaml"))
	layout, err := repository.ResolveDefault("", "workspace-1", "main")
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.PatchPlacements(layout.ID, []PlacementPatch{{NodeID: "plan", EntityRef: EntityRef{Kind: EntityPlan, WorkspaceID: "workspace-1", ItemID: "item-1", ItemPath: "plans/PM-037", BranchKey: "feature"}}})
	if err == nil {
		t.Fatal("expected branch validation error")
	}
	_, err = repository.PatchPlacements(layout.ID, []PlacementPatch{{NodeID: "workspace", EntityRef: EntityRef{Kind: EntityWorkspace, WorkspaceID: "workspace-1"}, ExpectedRevision: 1}})
	var conflict *PlacementConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want placement conflict", err)
	}
	_, err = repository.PatchPlacements(layout.ID, []PlacementPatch{{NodeID: "workspace", EntityRef: EntityRef{Kind: EntityWorkspace, WorkspaceID: "workspace-1"}, Position: Position{X: math.Inf(1)}}})
	if err == nil {
		t.Fatal("expected coordinate validation error")
	}
}

func TestFileRepositoryKeepsViewportVersionIndependent(t *testing.T) {
	repository := NewFileRepository(filepath.Join(t.TempDir(), "canvases.yaml"))
	layout, err := repository.ResolveDefault("", "workspace-1", "main")
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.PatchPlacements(layout.ID, []PlacementPatch{{NodeID: "workspace", EntityRef: EntityRef{Kind: EntityWorkspace, WorkspaceID: "workspace-1"}}})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := repository.SaveViewport(layout.ID, layout.Version, Viewport{X: 12, Y: 34, Zoom: 1.2})
	if err != nil || updated.Version != layout.Version+1 {
		t.Fatalf("viewport = %#v err=%v", updated, err)
	}
	placements, err := repository.Placements(layout.ID)
	if err != nil || placements[0].Revision != 1 {
		t.Fatalf("placements = %#v err=%v", placements, err)
	}
}
