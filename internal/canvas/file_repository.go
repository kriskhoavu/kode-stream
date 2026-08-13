package canvas

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const maxCanvasFileBytes int64 = 4 << 20

type FileRepository struct {
	path          string
	now           func() time.Time
	syncDirectory func(string) error
	mu            sync.Mutex
}

func NewFileRepository(path string) *FileRepository {
	return &FileRepository{path: path, now: time.Now, syncDirectory: syncCanvasDirectory}
}

func (r *FileRepository) ResolveDefault(ownerUserID, workspaceID, branchKey string) (Layout, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, err := r.load()
	if err != nil {
		return Layout{}, false, err
	}
	for _, layout := range state.Layouts {
		if layout.OwnerUserID == ownerUserID && layout.WorkspaceID == workspaceID && layout.BranchKey == branchKey {
			return layout, false, nil
		}
	}
	layout, err := NewLayout(ownerUserID, workspaceID, branchKey, r.now().UTC())
	if err != nil {
		return Layout{}, false, err
	}
	state.Layouts = append(state.Layouts, layout)
	return layout, true, r.save(state)
}

func (r *FileRepository) FindDefault(ownerUserID, workspaceID, branchKey string) (Layout, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, err := r.load()
	if err != nil {
		return Layout{}, false, err
	}
	for _, layout := range state.Layouts {
		if layout.OwnerUserID == ownerUserID && layout.WorkspaceID == workspaceID && layout.BranchKey == branchKey {
			return layout, true, nil
		}
	}
	return Layout{}, false, nil
}

func (r *FileRepository) InitializeDefault(ownerUserID, workspaceID, branchKey string, placements []Placement) (Layout, bool, error) {
	return r.InitializeDefaultContext(context.Background(), ownerUserID, workspaceID, branchKey, placements)
}

func (r *FileRepository) InitializeDefaultContext(ctx context.Context, ownerUserID, workspaceID, branchKey string, placements []Placement) (Layout, bool, error) {
	if err := ctx.Err(); err != nil {
		return Layout{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Layout{}, false, err
	}
	state, err := r.load()
	if err != nil {
		return Layout{}, false, err
	}
	for _, layout := range state.Layouts {
		if layout.OwnerUserID == ownerUserID && layout.WorkspaceID == workspaceID && layout.BranchKey == branchKey {
			return layout, false, nil
		}
	}
	layout, err := NewLayout(ownerUserID, workspaceID, branchKey, r.now().UTC())
	if err != nil {
		return Layout{}, false, err
	}
	if len(placements) == 0 || len(placements) > MaxPlacements {
		return Layout{}, false, errors.New("canvas initial placements are outside allowed limits")
	}
	next := Snapshot{Version: state.Version, Layouts: append(append([]Layout(nil), state.Layouts...), layout), Placements: append([]Placement(nil), state.Placements...)}
	for _, placement := range placements {
		placement.LayoutID = layout.ID
		next.Placements = append(next.Placements, placement)
	}
	if err := ValidateSnapshot(next); err != nil {
		return Layout{}, false, err
	}
	if err := ctx.Err(); err != nil {
		return Layout{}, false, err
	}
	if err := r.save(next); err != nil {
		return Layout{}, false, err
	}
	return layout, true, nil
}

func (r *FileRepository) GetLayout(id string) (Layout, bool, error) {
	return r.GetLayoutContext(context.Background(), id)
}
func (r *FileRepository) GetLayoutContext(ctx context.Context, id string) (Layout, bool, error) {
	if err := ctx.Err(); err != nil {
		return Layout{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Layout{}, false, err
	}
	state, err := r.load()
	if err != nil {
		return Layout{}, false, err
	}
	for _, layout := range state.Layouts {
		if layout.ID == id {
			return layout, true, nil
		}
	}
	return Layout{}, false, nil
}

func (r *FileRepository) Layouts() ([]Layout, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, err := r.load()
	if err != nil {
		return nil, err
	}
	result := append([]Layout(nil), state.Layouts...)
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (r *FileRepository) Placements(layoutID string) ([]Placement, error) {
	return r.PlacementsContext(context.Background(), layoutID)
}
func (r *FileRepository) PlacementsContext(ctx context.Context, layoutID string) ([]Placement, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	state, err := r.load()
	if err != nil {
		return nil, err
	}
	if !hasLayout(state, layoutID) {
		return nil, ErrNotFound
	}
	result := []Placement{}
	for _, placement := range state.Placements {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if placement.LayoutID == layoutID {
			result = append(result, placement)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].NodeID < result[j].NodeID })
	return result, nil
}

func (r *FileRepository) PatchPlacements(layoutID string, patches []PlacementPatch) ([]Placement, error) {
	return r.PatchPlacementsContext(context.Background(), layoutID, patches)
}
func (r *FileRepository) PatchPlacementsContext(ctx context.Context, layoutID string, patches []PlacementPatch) ([]Placement, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(patches) == 0 || len(patches) > MaxPlacementBatch {
		return nil, errors.New("canvas placement batch is outside allowed limits")
	}
	state, err := r.load()
	if err != nil {
		return nil, err
	}
	layout, ok := findLayout(state, layoutID)
	if !ok {
		return nil, ErrNotFound
	}
	updated, next, err := applyPlacementPatches(state, layout, patches, r.now().UTC())
	if err != nil {
		return nil, err
	}
	if err := r.save(next); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *FileRepository) RemovePlacement(layoutID, nodeID string, expectedRevision int64) error {
	return r.RemovePlacementContext(context.Background(), layoutID, nodeID, expectedRevision)
}
func (r *FileRepository) RemovePlacementContext(ctx context.Context, layoutID, nodeID string, expectedRevision int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	state, err := r.load()
	if err != nil {
		return err
	}
	if !hasLayout(state, layoutID) {
		return ErrNotFound
	}
	for i, placement := range state.Placements {
		if placement.LayoutID == layoutID && placement.NodeID == nodeID {
			if placement.Revision != expectedRevision {
				return &PlacementConflictError{NodeIDs: []string{nodeID}}
			}
			state.Placements[i].Hidden = true
			state.Placements[i].Revision++
			state.Placements[i].UpdatedAt = r.now().UTC()
			return r.save(state)
		}
	}
	return ErrNotFound
}

func (r *FileRepository) SaveViewport(layoutID string, expectedVersion int64, viewport Viewport) (Layout, error) {
	return r.SaveViewportContext(context.Background(), layoutID, expectedVersion, viewport)
}
func (r *FileRepository) SaveViewportContext(ctx context.Context, layoutID string, expectedVersion int64, viewport Viewport) (Layout, error) {
	if err := ctx.Err(); err != nil {
		return Layout{}, err
	}
	if err := validateViewport(viewport); err != nil {
		return Layout{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Layout{}, err
	}
	state, err := r.load()
	if err != nil {
		return Layout{}, err
	}
	for i := range state.Layouts {
		if state.Layouts[i].ID != layoutID {
			continue
		}
		if state.Layouts[i].Version != expectedVersion {
			return Layout{}, &PlacementConflictError{NodeIDs: []string{"viewport"}}
		}
		state.Layouts[i].Viewport = viewport
		state.Layouts[i].Version++
		state.Layouts[i].UpdatedAt = r.now().UTC()
		return state.Layouts[i], r.save(state)
	}
	return Layout{}, ErrNotFound
}

func (r *FileRepository) Snapshot() (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, err := r.load()
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Version: state.Version, Layouts: append([]Layout(nil), state.Layouts...), Placements: append([]Placement(nil), state.Placements...)}, nil
}

func (r *FileRepository) ReplaceAll(snapshot Snapshot) error {
	snapshot = normalizeSnapshot(snapshot)
	if err := ValidateSnapshot(snapshot); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.save(snapshot)
}

func (r *FileRepository) load() (Snapshot, error) {
	file, err := os.Open(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return normalizeSnapshot(Snapshot{}), nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxCanvasFileBytes+1))
	if err != nil {
		return Snapshot{}, err
	}
	if int64(len(data)) > maxCanvasFileBytes {
		return Snapshot{}, errors.New("canvas file exceeds the read limit")
	}
	var state Snapshot
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&state); err != nil {
		return Snapshot{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Snapshot{}, errors.New("canvas file contains multiple YAML documents")
	}
	state = normalizeSnapshot(state)
	return state, ValidateSnapshot(state)
}

func (r *FileRepository) save(state Snapshot) error {
	state = normalizeSnapshot(state)
	if err := ValidateSnapshot(state); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(r.path); statErr == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	temporary, err := os.CreateTemp(filepath.Dir(r.path), ".canvases-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	backupPath := ""
	if _, err := os.Stat(r.path); err == nil {
		backup, err := os.CreateTemp(filepath.Dir(r.path), ".canvases-previous-*")
		if err != nil {
			return err
		}
		backupPath = backup.Name()
		if err := backup.Close(); err != nil {
			return err
		}
		if err := os.Rename(r.path, backupPath); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(temporaryPath, r.path); err != nil {
		if backupPath != "" {
			_ = os.Rename(backupPath, r.path)
		}
		return err
	}
	if err := r.syncDirectory(filepath.Dir(r.path)); err != nil {
		rollbackErr := restoreCanvasGeneration(r.path, backupPath, r.syncDirectory)
		if rollbackErr != nil {
			return fmt.Errorf("canvas publication failed: %w (rollback failed: %v)", err, rollbackErr)
		}
		return err
	}
	if backupPath != "" {
		if err := os.Remove(backupPath); err != nil {
			return err
		}
		if err := r.syncDirectory(filepath.Dir(r.path)); err != nil {
			return err
		}
	}
	return nil
}

func syncCanvasDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func restoreCanvasGeneration(path, backupPath string, syncDirectory func(string) error) error {
	if backupPath == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else if err := os.Rename(backupPath, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func applyPlacementPatches(state Snapshot, layout Layout, patches []PlacementPatch, now time.Time) ([]Placement, Snapshot, error) {
	existing := map[string]int{}
	count := 0
	for i, placement := range state.Placements {
		if placement.LayoutID == layout.ID {
			existing[placement.NodeID] = i
			count++
		}
	}
	seen := map[string]bool{}
	conflicts := []string{}
	for _, patch := range patches {
		if seen[patch.NodeID] || patch.NodeID == "" {
			return nil, state, errors.New("canvas placement batch contains an invalid or duplicate node ID")
		}
		seen[patch.NodeID] = true
		if index, ok := existing[patch.NodeID]; ok {
			if state.Placements[index].Revision != patch.ExpectedRevision {
				conflicts = append(conflicts, patch.NodeID)
			}
		} else if patch.ExpectedRevision != 0 {
			conflicts = append(conflicts, patch.NodeID)
		}
	}
	if len(conflicts) > 0 {
		return nil, state, &PlacementConflictError{NodeIDs: conflicts}
	}
	updated := make([]Placement, 0, len(patches))
	for _, patch := range patches {
		placement := Placement{LayoutID: layout.ID, NodeID: patch.NodeID, EntityRef: patch.EntityRef, Position: patch.Position, Collapsed: patch.Collapsed, Revision: 1, UpdatedAt: now}
		if index, ok := existing[patch.NodeID]; ok {
			current := state.Placements[index]
			placement.Revision = current.Revision + 1
			placement.EntityRef = current.EntityRef
			placement.Hidden = current.Hidden
			if err := validatePlacement(layout, placement); err != nil {
				return nil, state, err
			}
			state.Placements[index] = placement
		} else {
			count++
			if count > MaxPlacements {
				return nil, state, errors.New("canvas layout exceeds placement limit")
			}
			if err := validatePlacement(layout, placement); err != nil {
				return nil, state, err
			}
			state.Placements = append(state.Placements, placement)
		}
		updated = append(updated, placement)
	}
	for i := range state.Layouts {
		if state.Layouts[i].ID == layout.ID {
			state.Layouts[i].UpdatedAt = now
			break
		}
	}
	return updated, state, nil
}

func findLayout(state Snapshot, id string) (Layout, bool) {
	for _, layout := range state.Layouts {
		if layout.ID == id {
			return layout, true
		}
	}
	return Layout{}, false
}

func hasLayout(state Snapshot, id string) bool {
	_, ok := findLayout(state, id)
	return ok
}
