package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"kode-stream/internal/common/models"
	itemindex "kode-stream/internal/item/index"
)

type SQLiteItemRepository struct {
	db     *sql.DB
	driver string
	mu     sync.Mutex
}

func (r *SQLiteItemRepository) ReplaceWorkspace(workspaceID string, items []models.ItemDetail, warnings []models.ScanWarning, scannedAt time.Time) error {
	if scannedAt.IsZero() {
		scannedAt = time.Now().UTC()
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	if _, err = execTx(tx, r.driver, `DELETE FROM indexed_items WHERE workspace_id = ?`, workspaceID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err = execTx(tx, r.driver, `DELETE FROM scan_warnings WHERE workspace_id = ?`, workspaceID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err = execTx(tx, r.driver, `DELETE FROM branch_scans WHERE workspace_id = ?`, workspaceID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err = insertItems(tx, r.driver, items, scannedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	branches := map[string]models.BranchScanMetadata{}
	for _, item := range items {
		branch := firstNonEmpty(item.Branch, "main")
		if _, ok := branches[branch]; !ok {
			branches[branch] = models.BranchScanMetadata{WorkspaceID: workspaceID, Branch: branch, BranchRef: item.BranchRef, Commit: item.Commit, SourceMode: firstNonEmpty(item.SourceMode, "working_tree"), Editable: item.Editable || item.SourceMode == "" || item.SourceMode == "working_tree", ScannedAt: scannedAt}
		}
	}
	for branch, metadata := range branches {
		if err = insertBranchScan(tx, r.driver, metadata); err != nil {
			_ = tx.Rollback()
			return err
		}
		_ = branch
	}
	for _, warning := range warnings {
		if _, err = execTx(tx, r.driver, `INSERT INTO scan_warnings (workspace_id, branch, item_path, code, message) VALUES (?, ?, ?, ?, ?)`, workspaceID, "", warning.ItemPath, "", warning.Message); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLiteItemRepository) ReplaceWorkspaceBranch(workspaceID, branch string, items []models.ItemDetail, metadata models.BranchScanMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	if _, err = execTx(tx, r.driver, `DELETE FROM indexed_items WHERE workspace_id = ? AND branch = ?`, workspaceID, branch); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err = execTx(tx, r.driver, `DELETE FROM scan_warnings WHERE workspace_id = ? AND branch = ?`, workspaceID, branch); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err = execTx(tx, r.driver, `DELETE FROM branch_scans WHERE workspace_id = ? AND branch = ?`, workspaceID, branch); err != nil {
		_ = tx.Rollback()
		return err
	}
	for i := range items {
		items[i].WorkspaceID = workspaceID
		items[i].Branch = branch
	}
	metadata.WorkspaceID, metadata.Branch = workspaceID, branch
	if metadata.ScannedAt.IsZero() {
		metadata.ScannedAt = time.Now().UTC()
	}
	if err = insertItems(tx, r.driver, items, metadata.ScannedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err = insertBranchScan(tx, r.driver, metadata); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, warning := range metadata.Warnings {
		if _, err = execTx(tx, r.driver, `INSERT INTO scan_warnings (workspace_id, branch, item_path, code, message) VALUES (?, ?, ?, ?, ?)`, workspaceID, branch, warning.ItemPath, "", warning.Message); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLiteItemRepository) DeleteWorkspace(workspaceID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{`DELETE FROM indexed_items WHERE workspace_id = ?`, `DELETE FROM branch_scans WHERE workspace_id = ?`, `DELETE FROM scan_warnings WHERE workspace_id = ?`} {
		if _, err := execTx(tx, r.driver, statement, workspaceID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLiteItemRepository) Query(q itemindex.Query) ([]models.ItemSummary, error) {
	details, err := r.queryDetails(q)
	if err != nil {
		return nil, err
	}
	text := strings.ToLower(strings.TrimSpace(q.Text))
	out := make([]models.ItemSummary, 0, len(details))
	for _, detail := range details {
		if text != "" && !summaryMatchesText(detail.ItemSummary, text) {
			continue
		}
		if detail.Tags == nil {
			detail.Tags = []string{}
		}
		out = append(out, detail.ItemSummary)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].UpdatedAt.After(out[b].UpdatedAt) })
	return out, nil
}

func (r *SQLiteItemRepository) VisitContext(ctx context.Context, q itemindex.Query, visit func(models.ItemSummary) bool) error {
	where := []string{"1=1"}
	args := []any{}
	if !q.IncludeSnapshots {
		where, args = append(where, "source_mode != ?"), append(args, "snapshot")
	}
	if q.WorkspaceID != "" {
		where, args = append(where, "workspace_id = ?"), append(args, q.WorkspaceID)
	}
	if q.Branch != "" {
		where, args = append(where, "branch = ?"), append(args, q.Branch)
	}
	if q.Status != "" {
		where, args = append(where, "status = ?"), append(args, q.Status)
	}
	rows, err := r.db.QueryContext(ctx, rebindSQL(r.driver, `SELECT metadata_json FROM indexed_items WHERE `+strings.Join(where, " AND ")+` ORDER BY updated_at DESC`), args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	text := strings.ToLower(strings.TrimSpace(q.Text))
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return err
		}
		var detail models.ItemDetail
		if err := json.Unmarshal([]byte(raw), &detail); err != nil {
			return err
		}
		if text != "" && !summaryMatchesText(detail.ItemSummary, text) {
			continue
		}
		if detail.Tags == nil {
			detail.Tags = []string{}
		}
		if !visit(detail.ItemSummary) {
			return nil
		}
	}
	return rows.Err()
}

func (r *SQLiteItemRepository) BranchItems(workspaceID, branch string) ([]models.ItemSummary, error) {
	return r.Query(itemindex.Query{WorkspaceID: workspaceID, Branch: branch, IncludeSnapshots: true})
}

func (r *SQLiteItemRepository) BranchScan(workspaceID, branch string) (models.BranchScanMetadata, bool, error) {
	var metadata models.BranchScanMetadata
	var scannedAt string
	err := queryRowSQL(r.db, r.driver, `SELECT workspace_id, branch, branch_ref, commit_sha, source_mode, editable, source_configuration_hash, working_tree_hash, scanned_at FROM branch_scans WHERE workspace_id = ? AND branch = ?`, workspaceID, branch).Scan(&metadata.WorkspaceID, &metadata.Branch, &metadata.BranchRef, &metadata.Commit, &metadata.SourceMode, &metadata.Editable, &metadata.SourceConfigurationHash, &metadata.WorkingTreeHash, &scannedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.BranchScanMetadata{}, false, nil
	}
	if err != nil {
		return models.BranchScanMetadata{}, false, err
	}
	metadata.ScannedAt = parseTime(scannedAt)
	warnings, err := r.branchWarnings(workspaceID, branch)
	if err != nil {
		return models.BranchScanMetadata{}, false, err
	}
	metadata.Warnings = warnings
	return metadata, true, nil
}

func (r *SQLiteItemRepository) Get(id string) (models.ItemDetail, bool, error) {
	var raw string
	err := queryRowSQL(r.db, r.driver, `SELECT metadata_json FROM indexed_items WHERE id = ? ORDER BY updated_at DESC LIMIT 1`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ItemDetail{}, false, nil
	}
	if err != nil {
		return models.ItemDetail{}, false, err
	}
	var item models.ItemDetail
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return models.ItemDetail{}, false, err
	}
	return item, true, nil
}

func (r *SQLiteItemRepository) queryDetails(q itemindex.Query) ([]models.ItemDetail, error) {
	where := []string{"1=1"}
	args := []any{}
	if !q.IncludeSnapshots {
		where = append(where, "source_mode != ?")
		args = append(args, "snapshot")
	}
	if q.WorkspaceID != "" {
		where = append(where, "workspace_id = ?")
		args = append(args, q.WorkspaceID)
	}
	if q.Branch != "" {
		where = append(where, "branch = ?")
		args = append(args, q.Branch)
	}
	if q.Status != "" {
		where = append(where, "status = ?")
		args = append(args, q.Status)
	}
	rows, err := querySQL(r.db, r.driver, `SELECT metadata_json FROM indexed_items WHERE `+strings.Join(where, " AND ")+` ORDER BY updated_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var details []models.ItemDetail
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item models.ItemDetail
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		details = append(details, item)
	}
	if details == nil {
		details = []models.ItemDetail{}
	}
	return details, rows.Err()
}

func (r *SQLiteItemRepository) branchWarnings(workspaceID, branch string) ([]models.ScanWarning, error) {
	rows, err := querySQL(r.db, r.driver, `SELECT item_path, code, message FROM scan_warnings WHERE workspace_id = ? AND branch = ?`, workspaceID, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var warnings []models.ScanWarning
	for rows.Next() {
		var warning models.ScanWarning
		var code string
		if err := rows.Scan(&warning.ItemPath, &code, &warning.Message); err != nil {
			return nil, err
		}
		warnings = append(warnings, warning)
	}
	if warnings == nil {
		warnings = []models.ScanWarning{}
	}
	return warnings, rows.Err()
}

func insertItems(tx *sql.Tx, driverName string, items []models.ItemDetail, fallbackUpdatedAt time.Time) error {
	if fallbackUpdatedAt.IsZero() {
		fallbackUpdatedAt = time.Now().UTC()
	}
	for _, item := range items {
		if item.SourceMode == "" {
			item.SourceMode = "working_tree"
			item.Editable = true
		}
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = fallbackUpdatedAt
		}
		raw, err := encodeJSON(item)
		if err != nil {
			return err
		}
		_, err = execTx(tx, driverName, `INSERT INTO indexed_items (id, workspace_id, branch, scope, identifier, title, status, item_path, source_mode, editable, metadata_json, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, item.WorkspaceID, item.Branch, item.Scope, item.Identifier, item.Title, item.Status, item.ItemPath, item.SourceMode, databaseBoolValue(driverName, item.Editable), raw, formatTime(item.UpdatedAt))
		if err != nil {
			return err
		}
	}
	return nil
}

func insertBranchScan(tx *sql.Tx, driverName string, metadata models.BranchScanMetadata) error {
	_, err := execTx(tx, driverName, `INSERT INTO branch_scans (workspace_id, branch, branch_ref, commit_sha, source_mode, editable, source_configuration_hash, working_tree_hash, scanned_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, metadata.WorkspaceID, metadata.Branch, metadata.BranchRef, metadata.Commit, metadata.SourceMode, databaseBoolValue(driverName, metadata.Editable), metadata.SourceConfigurationHash, metadata.WorkingTreeHash, formatTime(metadata.ScannedAt))
	return err
}

func summaryMatchesText(item models.ItemSummary, text string) bool {
	haystack := strings.ToLower(strings.Join([]string{
		item.Title, item.Identifier, item.Scope, item.Description, item.Author, strings.Join(item.Tags, " "),
	}, " "))
	return strings.Contains(haystack, text)
}
