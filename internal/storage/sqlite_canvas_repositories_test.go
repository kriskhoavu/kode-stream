package storage

import (
	"errors"
	"testing"
	"time"

	"kode-stream/internal/ai"
	"kode-stream/internal/canvas"
	"kode-stream/internal/common/models"
	"kode-stream/internal/system"
)

func TestSQLiteCanvasAndSessionRepositoryContract(t *testing.T) {
	paths := testPaths(t.TempDir())
	state, err := OpenAppOwnedState(paths, system.RuntimeConfig{Mode: models.RuntimeModeLocal}, nil, databaseEnv)
	if err != nil {
		t.Fatal(err)
	}
	defer state.SQLStore.Close()

	layout, err := state.Canvas.ResolveDefault("", "workspace-1", "main")
	if err != nil {
		t.Fatal(err)
	}
	placements, err := state.Canvas.PatchPlacements(layout.ID, []canvas.PlacementPatch{{NodeID: "workspace", EntityRef: canvas.EntityRef{Kind: canvas.EntityWorkspace, WorkspaceID: "workspace-1"}, Position: canvas.Position{X: 12, Y: 34}}})
	if err != nil || len(placements) != 1 || placements[0].Revision != 1 {
		t.Fatalf("placements = %#v err=%v", placements, err)
	}
	_, err = state.Canvas.PatchPlacements(layout.ID, []canvas.PlacementPatch{{NodeID: "workspace", Position: canvas.Position{X: 56, Y: 78}, ExpectedRevision: 0}})
	var conflict *canvas.PlacementConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want placement conflict", err)
	}
	if err := state.Canvas.RemovePlacement(layout.ID, "workspace", 1); err != nil {
		t.Fatal(err)
	}
	placements, err = state.Canvas.Placements(layout.ID)
	if err != nil || len(placements) != 1 || !placements[0].Hidden || placements[0].Revision != 2 {
		t.Fatalf("hidden placements = %#v err=%v", placements, err)
	}
	updatedLayout, err := state.Canvas.SaveViewport(layout.ID, layout.Version, canvas.Viewport{X: 1, Y: 2, Zoom: 1.25})
	if err != nil || updatedLayout.Version != 2 {
		t.Fatalf("layout = %#v err=%v", updatedLayout, err)
	}

	now := time.Now().UTC()
	record := ai.SessionRecord{ID: "session-1", WorkspaceID: "workspace-1", Provider: "codex", Intent: "implement", RequestedBranch: "main", State: ai.StateInterrupted, StartedAt: now, LastKnownAt: now}
	if _, err := state.SessionRecords.Upsert(record); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := state.SessionRecords.Get(record.ID)
	if err != nil || !ok || loaded.State != ai.StateInterrupted {
		t.Fatalf("record = %#v ok=%v err=%v", loaded, ok, err)
	}
}
