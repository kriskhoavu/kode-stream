package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"kode-stream/internal/ai"
	"kode-stream/internal/canvas"
)

type SQLiteCanvasRepository struct {
	db     *sql.DB
	driver string
	now    func() time.Time
}

type SQLiteSessionRecordRepository struct {
	db     *sql.DB
	driver string
}

func (r *SQLiteCanvasRepository) ResolveDefault(ownerUserID, workspaceID, branchKey string) (canvas.Layout, error) {
	created, err := canvas.NewLayout(ownerUserID, workspaceID, branchKey, r.now().UTC())
	if err != nil {
		return canvas.Layout{}, err
	}
	viewportJSON, _ := json.Marshal(created.Viewport)
	_, err = execSQL(r.db, r.driver, `INSERT INTO canvas_layouts (id, owner_user_id, workspace_id, branch_key, viewport_json, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(owner_user_id, workspace_id, branch_key) DO NOTHING`, created.ID, created.OwnerUserID, created.WorkspaceID, created.BranchKey, string(viewportJSON), created.Version, formatTime(created.CreatedAt), formatTime(created.UpdatedAt))
	if err != nil {
		return canvas.Layout{}, err
	}
	return r.getDefault(ownerUserID, workspaceID, branchKey)
}

func (r *SQLiteCanvasRepository) getDefault(ownerUserID, workspaceID, branchKey string) (canvas.Layout, error) {
	row := queryRowSQL(r.db, r.driver, `SELECT id, owner_user_id, workspace_id, branch_key, viewport_json, version, created_at, updated_at FROM canvas_layouts WHERE owner_user_id = ? AND workspace_id = ? AND branch_key = ?`, ownerUserID, workspaceID, branchKey)
	layout, err := scanCanvasLayout(row)
	if errors.Is(err, sql.ErrNoRows) {
		return canvas.Layout{}, canvas.ErrNotFound
	}
	return layout, err
}

func (r *SQLiteCanvasRepository) GetLayout(id string) (canvas.Layout, bool, error) {
	row := queryRowSQL(r.db, r.driver, `SELECT id, owner_user_id, workspace_id, branch_key, viewport_json, version, created_at, updated_at FROM canvas_layouts WHERE id = ?`, id)
	layout, err := scanCanvasLayout(row)
	if errors.Is(err, sql.ErrNoRows) {
		return canvas.Layout{}, false, nil
	}
	return layout, err == nil, err
}

func (r *SQLiteCanvasRepository) Layouts() ([]canvas.Layout, error) {
	rows, err := querySQL(r.db, r.driver, `SELECT id, owner_user_id, workspace_id, branch_key, viewport_json, version, created_at, updated_at FROM canvas_layouts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []canvas.Layout{}
	for rows.Next() {
		layout, err := scanCanvasLayout(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, layout)
	}
	return result, rows.Err()
}

func (r *SQLiteCanvasRepository) Placements(layoutID string) ([]canvas.Placement, error) {
	if _, ok, err := r.GetLayout(layoutID); err != nil {
		return nil, err
	} else if !ok {
		return nil, canvas.ErrNotFound
	}
	rows, err := querySQL(r.db, r.driver, `SELECT layout_id, node_id, entity_ref_json, x, y, collapsed, revision, updated_at FROM canvas_placements WHERE layout_id = ? ORDER BY node_id`, layoutID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []canvas.Placement{}
	for rows.Next() {
		placement, err := scanCanvasPlacement(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, placement)
	}
	return result, rows.Err()
}

func (r *SQLiteCanvasRepository) PatchPlacements(layoutID string, patches []canvas.PlacementPatch) ([]canvas.Placement, error) {
	if len(patches) == 0 || len(patches) > canvas.MaxPlacementBatch {
		return nil, errors.New("canvas placement batch is outside allowed limits")
	}
	layout, ok, err := r.GetLayout(layoutID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, canvas.ErrNotFound
	}
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRow(rebindSQL(r.driver, `SELECT COUNT(*) FROM canvas_placements WHERE layout_id = ?`), layoutID).Scan(&count); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	updated := make([]canvas.Placement, 0, len(patches))
	for _, patch := range patches {
		if strings.TrimSpace(patch.NodeID) == "" || seen[patch.NodeID] {
			return nil, errors.New("canvas placement batch contains an invalid or duplicate node ID")
		}
		seen[patch.NodeID] = true
		current, exists, err := getPlacementTx(tx, r.driver, layoutID, patch.NodeID)
		if err != nil {
			return nil, err
		}
		if exists && current.Revision != patch.ExpectedRevision || !exists && patch.ExpectedRevision != 0 {
			return nil, &canvas.PlacementConflictError{NodeIDs: []string{patch.NodeID}}
		}
		placement := canvas.Placement{LayoutID: layoutID, NodeID: patch.NodeID, EntityRef: patch.EntityRef, Position: patch.Position, Collapsed: patch.Collapsed, Revision: 1, UpdatedAt: r.now().UTC()}
		if exists {
			placement.Revision = current.Revision + 1
			if placement.EntityRef.Kind == "" {
				placement.EntityRef = current.EntityRef
			}
		} else {
			count++
			if count > canvas.MaxPlacements {
				return nil, errors.New("canvas layout exceeds placement limit")
			}
		}
		if err := canvas.ValidateSnapshot(canvas.Snapshot{Layouts: []canvas.Layout{layout}, Placements: []canvas.Placement{placement}}); err != nil {
			return nil, err
		}
		refJSON, _ := json.Marshal(placement.EntityRef)
		if exists {
			result, err := execTx(tx, r.driver, `UPDATE canvas_placements SET entity_ref_json = ?, x = ?, y = ?, collapsed = ?, revision = ?, updated_at = ? WHERE layout_id = ? AND node_id = ? AND revision = ?`, string(refJSON), placement.Position.X, placement.Position.Y, boolInt(placement.Collapsed), placement.Revision, formatTime(placement.UpdatedAt), layoutID, patch.NodeID, patch.ExpectedRevision)
			if err != nil {
				return nil, err
			}
			if affected, _ := result.RowsAffected(); affected != 1 {
				return nil, &canvas.PlacementConflictError{NodeIDs: []string{patch.NodeID}}
			}
		} else {
			_, err := execTx(tx, r.driver, `INSERT INTO canvas_placements (layout_id, node_id, entity_ref_json, x, y, collapsed, revision, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, layoutID, patch.NodeID, string(refJSON), placement.Position.X, placement.Position.Y, boolInt(placement.Collapsed), placement.Revision, formatTime(placement.UpdatedAt))
			if err != nil {
				return nil, err
			}
		}
		updated = append(updated, placement)
	}
	if _, err := execTx(tx, r.driver, `UPDATE canvas_layouts SET updated_at = ? WHERE id = ?`, formatTime(r.now().UTC()), layoutID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *SQLiteCanvasRepository) RemovePlacement(layoutID, nodeID string, expectedRevision int64) error {
	result, err := execSQL(r.db, r.driver, `DELETE FROM canvas_placements WHERE layout_id = ? AND node_id = ? AND revision = ?`, layoutID, nodeID, expectedRevision)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return &canvas.PlacementConflictError{NodeIDs: []string{nodeID}}
	}
	return nil
}

func (r *SQLiteCanvasRepository) SaveViewport(layoutID string, expectedVersion int64, viewport canvas.Viewport) (canvas.Layout, error) {
	layout, ok, err := r.GetLayout(layoutID)
	if err != nil {
		return canvas.Layout{}, err
	}
	if !ok {
		return canvas.Layout{}, canvas.ErrNotFound
	}
	layout.Viewport = viewport
	layout.Version++
	layout.UpdatedAt = r.now().UTC()
	if err := canvas.ValidateSnapshot(canvas.Snapshot{Layouts: []canvas.Layout{layout}}); err != nil {
		return canvas.Layout{}, err
	}
	viewportJSON, _ := json.Marshal(viewport)
	result, err := execSQL(r.db, r.driver, `UPDATE canvas_layouts SET viewport_json = ?, version = ?, updated_at = ? WHERE id = ? AND version = ?`, string(viewportJSON), layout.Version, formatTime(layout.UpdatedAt), layoutID, expectedVersion)
	if err != nil {
		return canvas.Layout{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return canvas.Layout{}, &canvas.PlacementConflictError{NodeIDs: []string{"viewport"}}
	}
	return layout, nil
}

func (r *SQLiteCanvasRepository) Snapshot() (canvas.Snapshot, error) {
	layouts, err := r.Layouts()
	if err != nil {
		return canvas.Snapshot{}, err
	}
	placements := []canvas.Placement{}
	for _, layout := range layouts {
		items, err := r.Placements(layout.ID)
		if err != nil {
			return canvas.Snapshot{}, err
		}
		placements = append(placements, items...)
	}
	return canvas.Snapshot{Layouts: layouts, Placements: placements}, nil
}

func (r *SQLiteCanvasRepository) ReplaceAll(snapshot canvas.Snapshot) error {
	if err := canvas.ValidateSnapshot(snapshot); err != nil {
		return err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"canvas_placements", "canvas_layouts"} {
		if _, err := execTx(tx, r.driver, "DELETE FROM "+table); err != nil {
			return err
		}
	}
	for _, layout := range snapshot.Layouts {
		viewportJSON, _ := json.Marshal(layout.Viewport)
		if _, err := execTx(tx, r.driver, `INSERT INTO canvas_layouts (id, owner_user_id, workspace_id, branch_key, viewport_json, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, layout.ID, layout.OwnerUserID, layout.WorkspaceID, layout.BranchKey, string(viewportJSON), layout.Version, formatTime(layout.CreatedAt), formatTime(layout.UpdatedAt)); err != nil {
			return err
		}
	}
	for _, placement := range snapshot.Placements {
		refJSON, _ := json.Marshal(placement.EntityRef)
		if _, err := execTx(tx, r.driver, `INSERT INTO canvas_placements (layout_id, node_id, entity_ref_json, x, y, collapsed, revision, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, placement.LayoutID, placement.NodeID, string(refJSON), placement.Position.X, placement.Position.Y, boolInt(placement.Collapsed), placement.Revision, formatTime(placement.UpdatedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanCanvasLayout(row sqlRow) (canvas.Layout, error) {
	var layout canvas.Layout
	var viewportJSON, createdAt, updatedAt string
	err := row.Scan(&layout.ID, &layout.OwnerUserID, &layout.WorkspaceID, &layout.BranchKey, &viewportJSON, &layout.Version, &createdAt, &updatedAt)
	if err != nil {
		return canvas.Layout{}, err
	}
	if err := json.Unmarshal([]byte(viewportJSON), &layout.Viewport); err != nil {
		return canvas.Layout{}, err
	}
	layout.CreatedAt, layout.UpdatedAt = parseTime(createdAt), parseTime(updatedAt)
	return layout, nil
}

func scanCanvasPlacement(row sqlRow) (canvas.Placement, error) {
	var placement canvas.Placement
	var refJSON, updatedAt string
	var collapsed any
	err := row.Scan(&placement.LayoutID, &placement.NodeID, &refJSON, &placement.Position.X, &placement.Position.Y, &collapsed, &placement.Revision, &updatedAt)
	if err != nil {
		return canvas.Placement{}, err
	}
	if err := json.Unmarshal([]byte(refJSON), &placement.EntityRef); err != nil {
		return canvas.Placement{}, err
	}
	placement.Collapsed = databaseBool(collapsed)
	placement.UpdatedAt = parseTime(updatedAt)
	return placement, nil
}

func getPlacementTx(tx *sql.Tx, driver, layoutID, nodeID string) (canvas.Placement, bool, error) {
	row := tx.QueryRow(rebindSQL(driver, `SELECT layout_id, node_id, entity_ref_json, x, y, collapsed, revision, updated_at FROM canvas_placements WHERE layout_id = ? AND node_id = ?`), layoutID, nodeID)
	placement, err := scanCanvasPlacement(row)
	if errors.Is(err, sql.ErrNoRows) {
		return canvas.Placement{}, false, nil
	}
	return placement, err == nil, err
}

func databaseBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int64:
		return typed != 0
	case int:
		return typed != 0
	case []byte:
		return string(typed) == "1" || strings.EqualFold(string(typed), "true")
	case string:
		return typed == "1" || strings.EqualFold(typed, "true")
	default:
		return false
	}
}

func (r *SQLiteSessionRecordRepository) Get(id string) (ai.SessionRecord, bool, error) {
	record, err := scanSessionRecord(queryRowSQL(r.db, r.driver, sessionRecordSelect+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ai.SessionRecord{}, false, nil
	}
	return record, err == nil, err
}

func (r *SQLiteSessionRecordRepository) FindByIdempotency(workspaceID, key string) (ai.SessionRecord, bool, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(key) == "" {
		return ai.SessionRecord{}, false, nil
	}
	record, err := scanSessionRecord(queryRowSQL(r.db, r.driver, sessionRecordSelect+` WHERE workspace_id = ? AND idempotency_key = ?`, workspaceID, key))
	if errors.Is(err, sql.ErrNoRows) {
		return ai.SessionRecord{}, false, nil
	}
	return record, err == nil, err
}

func (r *SQLiteSessionRecordRepository) List(workspaceID, branch string) ([]ai.SessionRecord, error) {
	query := sessionRecordSelect + ` WHERE (? = '' OR workspace_id = ?) AND (? = '' OR requested_branch = ?) ORDER BY started_at DESC, id`
	rows, err := querySQL(r.db, r.driver, query, workspaceID, workspaceID, branch, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ai.SessionRecord{}
	for rows.Next() {
		record, err := scanSessionRecord(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, rows.Err()
}

func (r *SQLiteSessionRecordRepository) Upsert(record ai.SessionRecord) (ai.SessionRecord, error) {
	if err := ai.ValidateSessionRecord(record); err != nil {
		return ai.SessionRecord{}, err
	}
	planJSON := ""
	if record.PlanRef != nil {
		data, _ := json.Marshal(record.PlanRef)
		planJSON = string(data)
	}
	_, err := execSQL(r.db, r.driver, `INSERT INTO ai_session_records (id, workspace_id, plan_ref_json, provider, intent, requested_branch, observed_commit, idempotency_key, state, started_at, ended_at, exit_code, last_known_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET plan_ref_json = excluded.plan_ref_json, provider = excluded.provider, intent = excluded.intent, requested_branch = excluded.requested_branch, observed_commit = excluded.observed_commit, idempotency_key = excluded.idempotency_key, state = excluded.state, started_at = excluded.started_at, ended_at = excluded.ended_at, exit_code = excluded.exit_code, last_known_at = excluded.last_known_at`, record.ID, record.WorkspaceID, planJSON, record.Provider, record.Intent, record.RequestedBranch, record.ObservedCommit, record.IdempotencyKey, record.State, formatTime(record.StartedAt), formatTime(record.EndedAt), record.ExitCode, formatTime(record.LastKnownAt))
	return record, err
}

func (r *SQLiteSessionRecordRepository) Snapshot() ([]ai.SessionRecord, error) {
	return r.List("", "")
}

func (r *SQLiteSessionRecordRepository) ReplaceAll(records []ai.SessionRecord) error {
	seen := map[string]bool{}
	for _, record := range records {
		if err := ai.ValidateSessionRecord(record); err != nil {
			return err
		}
		if seen[record.ID] {
			return fmt.Errorf("duplicate session record ID %q", record.ID)
		}
		seen[record.ID] = true
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := execTx(tx, r.driver, `DELETE FROM ai_session_records`); err != nil {
		return err
	}
	for _, record := range records {
		planJSON := ""
		if record.PlanRef != nil {
			data, _ := json.Marshal(record.PlanRef)
			planJSON = string(data)
		}
		var exitCode any
		if record.ExitCode != nil {
			exitCode = *record.ExitCode
		}
		if _, err := execTx(tx, r.driver, `INSERT INTO ai_session_records (id, workspace_id, plan_ref_json, provider, intent, requested_branch, observed_commit, idempotency_key, state, started_at, ended_at, exit_code, last_known_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, record.ID, record.WorkspaceID, planJSON, record.Provider, record.Intent, record.RequestedBranch, record.ObservedCommit, record.IdempotencyKey, record.State, formatTime(record.StartedAt), formatTime(record.EndedAt), exitCode, formatTime(record.LastKnownAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const sessionRecordSelect = `SELECT id, workspace_id, plan_ref_json, provider, intent, requested_branch, observed_commit, idempotency_key, state, started_at, ended_at, exit_code, last_known_at FROM ai_session_records`

func scanSessionRecord(row sqlRow) (ai.SessionRecord, error) {
	var record ai.SessionRecord
	var planJSON string
	var startedAt, lastKnownAt string
	var endedAt sql.NullString
	var exitCode sql.NullInt64
	err := row.Scan(&record.ID, &record.WorkspaceID, &planJSON, &record.Provider, &record.Intent, &record.RequestedBranch, &record.ObservedCommit, &record.IdempotencyKey, &record.State, &startedAt, &endedAt, &exitCode, &lastKnownAt)
	if err != nil {
		return ai.SessionRecord{}, err
	}
	if planJSON != "" {
		var plan ai.SessionPlanRef
		if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
			return ai.SessionRecord{}, err
		}
		record.PlanRef = &plan
	}
	record.StartedAt, record.LastKnownAt = parseTime(startedAt), parseTime(lastKnownAt)
	if endedAt.Valid {
		record.EndedAt = parseTime(endedAt.String)
	}
	if exitCode.Valid {
		value := int(exitCode.Int64)
		record.ExitCode = &value
	}
	return record, nil
}

func sortSessionRecords(records []ai.SessionRecord) {
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
}
