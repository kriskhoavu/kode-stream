package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"kode-stream/internal/ai"
	"kode-stream/internal/audit"
	"kode-stream/internal/canvas"
	"kode-stream/internal/common/models"
	appgit "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/navigation"
	"kode-stream/internal/system"
	"kode-stream/internal/workspace/registry"
)

type sqlRow interface {
	Scan(...any) error
}

func execSQL(db *sql.DB, driverName, query string, args ...any) (sql.Result, error) {
	return db.Exec(rebindSQL(driverName, query), args...)
}

func querySQL(db *sql.DB, driverName, query string, args ...any) (*sql.Rows, error) {
	return db.Query(rebindSQL(driverName, query), args...)
}

func queryRowSQL(db *sql.DB, driverName, query string, args ...any) sqlRow {
	return db.QueryRow(rebindSQL(driverName, query), args...)
}

func execTx(tx *sql.Tx, driverName, query string, args ...any) (sql.Result, error) {
	return tx.Exec(rebindSQL(driverName, query), args...)
}

func rebindSQL(driverName, query string) string {
	if driverName != StorageDriverPostgres {
		return query
	}
	var builder strings.Builder
	ordinal := 1
	for _, char := range query {
		if char == '?' {
			builder.WriteString(fmt.Sprintf("$%d", ordinal))
			ordinal++
			continue
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

type SQLiteWorkspaceRepository struct {
	db        *sql.DB
	driver    string
	path      string
	validator *registry.Registry
	mu        sync.Mutex
}

type SQLiteItemRepository struct {
	db     *sql.DB
	driver string
	mu     sync.Mutex
}

type SQLiteAuditRepository struct {
	db     *sql.DB
	driver string
	now    func() time.Time
	mu     sync.Mutex
}

type SQLiteNavigationRepository struct {
	db     *sql.DB
	driver string
	now    func() time.Time
	mu     sync.Mutex
}

type SQLiteAISettingsRepository struct {
	db     *sql.DB
	driver string
	mu     sync.Mutex
}

type SQLiteImportStatusRepository struct {
	db     *sql.DB
	driver string
}

func newSQLiteWorkspaceRepository(store *SQLStore, paths system.Paths, git *appgit.GitAdapter) *SQLiteWorkspaceRepository {
	return &SQLiteWorkspaceRepository{db: store.db, driver: store.driver, path: paths.RegistryFile, validator: registry.New(paths.RegistryFile, git)}
}

func (r *SQLiteWorkspaceRepository) List() ([]models.WorkspaceConfig, error) {
	rows, err := querySQL(r.db, r.driver, `SELECT workspace_json FROM workspaces ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var workspaces []models.WorkspaceConfig
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var workspace models.WorkspaceConfig
		if err := json.Unmarshal([]byte(raw), &workspace); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, normalizeWorkspaceForSQL(workspace))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if workspaces == nil {
		workspaces = []models.WorkspaceConfig{}
	}
	return workspaces, nil
}

func (r *SQLiteWorkspaceRepository) Get(id string) (models.WorkspaceConfig, bool, error) {
	var raw string
	err := queryRowSQL(r.db, r.driver, `SELECT workspace_json FROM workspaces WHERE id = ?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return models.WorkspaceConfig{}, false, nil
	}
	if err != nil {
		return models.WorkspaceConfig{}, false, err
	}
	var workspace models.WorkspaceConfig
	if err := json.Unmarshal([]byte(raw), &workspace); err != nil {
		return models.WorkspaceConfig{}, false, err
	}
	return normalizeWorkspaceForSQL(workspace), true, nil
}

func (r *SQLiteWorkspaceRepository) Create(input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	workspace, err := r.Validate(input)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, err := r.List()
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	for _, candidate := range existing {
		if sameWorkspacePath(candidate.Path, workspace.Path) {
			return models.WorkspaceConfig{}, fmt.Errorf("workspace already registered")
		}
	}
	return workspace, r.upsert(workspace)
}

func (r *SQLiteWorkspaceRepository) Validate(input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	return r.validator.Validate(input)
}

func (r *SQLiteWorkspaceRepository) Path() string { return r.path }

func (r *SQLiteWorkspaceRepository) BatchCreate(inputs []models.WorkspaceInput) ([]registry.BatchCreateResult, error) {
	results := make([]registry.BatchCreateResult, len(inputs))
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, err := r.List()
	if err != nil {
		return nil, err
	}
	accepted := append([]models.WorkspaceConfig(nil), existing...)
	for i, input := range inputs {
		workspace, err := r.Validate(input)
		if err != nil {
			results[i].Err = err
			continue
		}
		duplicate := false
		for _, candidate := range accepted {
			if sameWorkspacePath(candidate.Path, workspace.Path) {
				duplicate = true
				break
			}
		}
		if duplicate {
			results[i].Err = fmt.Errorf("workspace already registered")
			continue
		}
		if err := r.upsert(workspace); err != nil {
			return results, err
		}
		accepted = append(accepted, workspace)
		results[i].Workspace = normalizeWorkspaceForSQL(workspace)
	}
	return results, nil
}

func (r *SQLiteWorkspaceRepository) Update(id string, input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	existing, ok, err := r.Get(id)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	if !ok {
		return models.WorkspaceConfig{}, fmt.Errorf("workspace not found")
	}
	if strings.TrimSpace(string(input.RegistrationMode)) == "" {
		input.RegistrationMode = existing.RegistrationMode
	}
	if strings.TrimSpace(input.RemoteURL) == "" {
		input.RemoteURL = existing.RemoteURL
	}
	if input.Knowledge == nil {
		input.Knowledge = existing.Knowledge
	}
	if input.Runtime == nil {
		input.Runtime = existing.Runtime
	}
	workspace, err := r.Validate(input)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	workspace.ID = existing.ID
	workspace.CreatedAt = existing.CreatedAt
	workspace.LastScannedAt = existing.LastScannedAt
	workspace.LastSelectedBranch = existing.LastSelectedBranch
	r.mu.Lock()
	defer r.mu.Unlock()
	workspaces, err := r.List()
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	for _, candidate := range workspaces {
		if candidate.ID != id && sameWorkspacePath(candidate.Path, workspace.Path) {
			return models.WorkspaceConfig{}, fmt.Errorf("workspace already registered")
		}
	}
	return normalizeWorkspaceForSQL(workspace), r.upsert(workspace)
}

func (r *SQLiteWorkspaceRepository) Delete(id string) error {
	result, err := execSQL(r.db, r.driver, `DELETE FROM workspaces WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("workspace not found")
	}
	return nil
}

// DeleteWorkspaceState removes every SQL-owned projection in one transaction.
// It is used by the workspace application service instead of coordinating two
// independently committing repositories.
func (r *SQLiteWorkspaceRepository) DeleteWorkspaceState(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`DELETE FROM scan_warnings WHERE workspace_id = ?`,
		`DELETE FROM branch_scans WHERE workspace_id = ?`,
		`DELETE FROM indexed_items WHERE workspace_id = ?`,
		`DELETE FROM recent_items WHERE workspace_id = ?`,
		`DELETE FROM canvas_placements WHERE layout_id IN (SELECT id FROM canvas_layouts WHERE workspace_id = ?)`,
		`DELETE FROM canvas_layouts WHERE workspace_id = ?`,
		`DELETE FROM ai_session_records WHERE workspace_id = ?`,
	} {
		if _, err := execTx(tx, r.driver, statement, id); err != nil {
			return err
		}
	}
	result, err := execTx(tx, r.driver, `DELETE FROM workspaces WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows == 0 {
		if err != nil {
			return err
		}
		return fmt.Errorf("workspace not found")
	}
	return tx.Commit()
}

func (r *SQLiteWorkspaceRepository) TouchScanned(id string, scannedAt time.Time) error {
	return r.patch(id, func(workspace *models.WorkspaceConfig) { workspace.LastScannedAt = scannedAt })
}

func (r *SQLiteWorkspaceRepository) SetLastSelectedBranch(id, branch string) error {
	return r.patch(id, func(workspace *models.WorkspaceConfig) { workspace.LastSelectedBranch = strings.TrimSpace(branch) })
}

func (r *SQLiteWorkspaceRepository) SetRuntime(id string, runtimeConfig *models.WorkspaceRuntimeConfig) (models.WorkspaceConfig, error) {
	var out models.WorkspaceConfig
	err := r.patch(id, func(workspace *models.WorkspaceConfig) {
		workspace.Runtime = runtimeConfig
		out = *workspace
	})
	return out, err
}

func (r *SQLiteWorkspaceRepository) patch(id string, apply func(*models.WorkspaceConfig)) error {
	workspace, ok, err := r.Get(id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("workspace not found")
	}
	apply(&workspace)
	return r.upsert(workspace)
}

func (r *SQLiteWorkspaceRepository) upsert(workspace models.WorkspaceConfig) error {
	workspace = normalizeWorkspaceForSQL(workspace)
	workspaceJSON, err := encodeJSON(workspace)
	if err != nil {
		return err
	}
	sourcesJSON, err := encodeJSON(workspace.Sources)
	if err != nil {
		return err
	}
	runtimeJSON, err := encodeJSON(workspace.Runtime)
	if err != nil {
		return err
	}
	_, err = execSQL(r.db, r.driver, `INSERT INTO workspaces (id, name, path_label, baseline_branch, registration_mode, remote_url, clone_path_managed, last_selected_branch, sources_json, runtime_json, workspace_json, created_at, last_scanned_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET name = excluded.name, path_label = excluded.path_label, baseline_branch = excluded.baseline_branch, registration_mode = excluded.registration_mode, remote_url = excluded.remote_url, clone_path_managed = excluded.clone_path_managed, last_selected_branch = excluded.last_selected_branch, sources_json = excluded.sources_json, runtime_json = excluded.runtime_json, workspace_json = excluded.workspace_json, created_at = excluded.created_at, last_scanned_at = excluded.last_scanned_at`,
		workspace.ID, workspace.Name, workspacePathLabel(workspace), workspace.BaselineBranch, workspace.RegistrationMode, workspace.RemoteURL, databaseBoolValue(r.driver, workspace.ClonePathManaged), workspace.LastSelectedBranch, sourcesJSON, runtimeJSON, workspaceJSON, formatTime(workspace.CreatedAt), formatTime(workspace.LastScannedAt))
	return err
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

func (r *SQLiteAuditRepository) Append(event models.AuditEvent) (models.AuditEvent, error) {
	if event.ID == "" {
		event.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if event.Time.IsZero() {
		event.Time = r.now().UTC()
	}
	if event.Paths == nil {
		event.Paths = []string{}
	}
	pathsJSON, err := encodeJSON(event.Paths)
	if err != nil {
		return models.AuditEvent{}, err
	}
	_, err = execSQL(r.db, r.driver, `INSERT INTO audit_events (id, owner_user_id, actor_user_id, workspace_id, item_id, operation, status, message, paths_json, duration_ms, error, event_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, event.ID, event.OwnerUserID, event.ActorUserID, event.WorkspaceID, event.ItemID, event.Operation, event.Status, event.Message, pathsJSON, event.DurationMS, event.Error, formatTime(event.Time))
	return event, err
}

func (r *SQLiteAuditRepository) Recent(limit int) ([]models.AuditEvent, error) {
	return r.QueryContext(context.Background(), audit.Query{Limit: limit})
}

func (r *SQLiteAuditRepository) RecentContext(ctx context.Context, limit int) ([]models.AuditEvent, error) {
	return r.QueryContext(ctx, audit.Query{Limit: limit})
}

func (r *SQLiteAuditRepository) QueryContext(ctx context.Context, q audit.Query) ([]models.AuditEvent, error) {
	where := []string{"1=1"}
	args := []any{}
	if q.OwnerUserID != "" {
		where = append(where, "owner_user_id = ?")
		args = append(args, q.OwnerUserID)
	}
	if q.WorkspaceID != "" {
		where = append(where, "workspace_id = ?")
		args = append(args, q.WorkspaceID)
	}
	query := `SELECT id, owner_user_id, actor_user_id, workspace_id, item_id, operation, status, message, paths_json, duration_ms, error, event_time FROM audit_events WHERE ` + strings.Join(where, " AND ") + ` ORDER BY event_time DESC`
	if q.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, q.Limit)
	}
	rows, err := querySQL(r.db, r.driver, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []models.AuditEvent{}
	for rows.Next() {
		var event models.AuditEvent
		var pathsJSON, eventTime string
		if err := rows.Scan(&event.ID, &event.OwnerUserID, &event.ActorUserID, &event.WorkspaceID, &event.ItemID, &event.Operation, &event.Status, &event.Message, &pathsJSON, &event.DurationMS, &event.Error, &eventTime); err != nil {
			return nil, err
		}
		event.Time = parseTime(eventTime)
		_ = json.Unmarshal([]byte(pathsJSON), &event.Paths)
		if event.Paths == nil {
			event.Paths = []string{}
		}
		event = audit.NormalizeLegacyEvent(event)
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *SQLiteNavigationRepository) Filters() ([]models.SavedFilter, error) {
	return r.filtersForOwner("")
}

func (r *SQLiteNavigationRepository) FiltersForOwner(owner string) ([]models.SavedFilter, error) {
	return r.filtersForOwner(normalizeNavigationOwner(owner))
}

func (r *SQLiteNavigationRepository) filtersForOwner(owner string) ([]models.SavedFilter, error) {
	query := `SELECT owner_user_id, id, name, route, workspace_id, filters_json, created_at, updated_at FROM saved_filters`
	args := []any{}
	if owner != "" {
		query += ` WHERE owner_user_id = ?`
		args = append(args, owner)
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := querySQL(r.db, r.driver, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var filters []models.SavedFilter
	for rows.Next() {
		var filter models.SavedFilter
		var filtersJSON string
		var createdAt, updatedAt string
		if err := rows.Scan(&filter.OwnerUserID, &filter.ID, &filter.Name, &filter.Route, &filter.WorkspaceID, &filtersJSON, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		filter.CreatedAt = parseTime(createdAt)
		filter.UpdatedAt = parseTime(updatedAt)
		_ = json.Unmarshal([]byte(filtersJSON), &filter.Filters)
		if filter.Filters == nil {
			filter.Filters = map[string]any{}
		}
		filters = append(filters, filter)
	}
	if filters == nil {
		filters = []models.SavedFilter{}
	}
	return filters, rows.Err()
}

func (r *SQLiteNavigationRepository) SaveFilter(filter models.SavedFilter) (models.SavedFilter, error) {
	return r.saveFilterForOwner(normalizeNavigationOwner(filter.OwnerUserID), filter)
}

func (r *SQLiteNavigationRepository) SaveFilterForOwner(owner string, filter models.SavedFilter) (models.SavedFilter, error) {
	return r.saveFilterForOwner(normalizeNavigationOwner(owner), filter)
}

func (r *SQLiteNavigationRepository) saveFilterForOwner(owner string, filter models.SavedFilter) (models.SavedFilter, error) {
	now := r.now().UTC()
	filter.OwnerUserID = owner
	if filter.ID == "" {
		filter.ID = navigationID()
		filter.CreatedAt = now
	}
	if filter.CreatedAt.IsZero() {
		filter.CreatedAt = now
	}
	filter.UpdatedAt = now
	if filter.Filters == nil {
		filter.Filters = map[string]any{}
	}
	filtersJSON, err := encodeJSON(filter.Filters)
	if err != nil {
		return models.SavedFilter{}, err
	}
	_, err = execSQL(r.db, r.driver, `INSERT INTO saved_filters (owner_user_id, id, name, route, workspace_id, filters_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(owner_user_id, id) DO UPDATE SET name = excluded.name, route = excluded.route, workspace_id = excluded.workspace_id, filters_json = excluded.filters_json, updated_at = excluded.updated_at`, owner, filter.ID, filter.Name, filter.Route, filter.WorkspaceID, filtersJSON, formatTime(filter.CreatedAt), formatTime(filter.UpdatedAt))
	return filter, err
}

func (r *SQLiteNavigationRepository) DeleteFilter(id string) (bool, error) {
	return r.deleteFilterForOwner("", id)
}

func (r *SQLiteNavigationRepository) DeleteFilterForOwner(owner, id string) (bool, error) {
	return r.deleteFilterForOwner(normalizeNavigationOwner(owner), id)
}

func (r *SQLiteNavigationRepository) deleteFilterForOwner(owner, id string) (bool, error) {
	query := `DELETE FROM saved_filters WHERE id = ?`
	args := []any{id}
	if owner != "" {
		query += ` AND owner_user_id = ?`
		args = append(args, owner)
	}
	result, err := execSQL(r.db, r.driver, query, args...)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func (r *SQLiteNavigationRepository) Recents(limit int) ([]models.RecentItem, error) {
	return r.recentsForOwner("", limit)
}

func (r *SQLiteNavigationRepository) RecentsForOwner(owner string, limit int) ([]models.RecentItem, error) {
	return r.recentsForOwner(normalizeNavigationOwner(owner), limit)
}

func (r *SQLiteNavigationRepository) recentsForOwner(owner string, limit int) ([]models.RecentItem, error) {
	query := `SELECT owner_user_id, item_id, workspace_id, title, subtitle, route, opened_at FROM recent_items`
	args := []any{}
	if owner != "" {
		query += ` WHERE owner_user_id = ?`
		args = append(args, owner)
	}
	query += ` ORDER BY opened_at DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := querySQL(r.db, r.driver, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var recents []models.RecentItem
	for rows.Next() {
		var item models.RecentItem
		var openedAt string
		if err := rows.Scan(&item.OwnerUserID, &item.ItemID, &item.WorkspaceID, &item.Title, &item.Subtitle, &item.Route, &openedAt); err != nil {
			return nil, err
		}
		item.OpenedAt = parseTime(openedAt)
		recents = append(recents, item)
	}
	if recents == nil {
		recents = []models.RecentItem{}
	}
	return recents, rows.Err()
}

func (r *SQLiteNavigationRepository) RecordRecent(item models.RecentItem) error {
	return r.recordRecentForOwner(normalizeNavigationOwner(item.OwnerUserID), item)
}

func (r *SQLiteNavigationRepository) RecordRecentForOwner(owner string, item models.RecentItem) error {
	return r.recordRecentForOwner(normalizeNavigationOwner(owner), item)
}

func (r *SQLiteNavigationRepository) recordRecentForOwner(owner string, item models.RecentItem) error {
	item.OwnerUserID = owner
	item.OpenedAt = r.now().UTC()
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = execTx(tx, r.driver, `INSERT INTO recent_items (owner_user_id, item_id, workspace_id, title, subtitle, route, opened_at) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(owner_user_id, item_id) DO UPDATE SET workspace_id = excluded.workspace_id, title = excluded.title, subtitle = excluded.subtitle, route = excluded.route, opened_at = excluded.opened_at`, owner, item.ItemID, item.WorkspaceID, item.Title, item.Subtitle, item.Route, formatTime(item.OpenedAt)); err != nil {
		return err
	}
	if _, err = execTx(tx, r.driver, `DELETE FROM recent_items WHERE owner_user_id = ? AND item_id NOT IN (SELECT item_id FROM recent_items WHERE owner_user_id = ? ORDER BY opened_at DESC LIMIT 50)`, owner, owner); err != nil {
		return err
	}
	return tx.Commit()
}

// RestoreFilter and RestoreRecent preserve immutable snapshot identity and
// timestamps; user-facing mutations intentionally continue to advance time.
func (r *SQLiteNavigationRepository) RestoreFilter(filter models.SavedFilter) error {
	owner := normalizeNavigationOwner(filter.OwnerUserID)
	if filter.ID == "" {
		return errors.New("saved filter snapshot id is required")
	}
	filtersJSON, err := encodeJSON(filter.Filters)
	if err != nil {
		return err
	}
	_, err = execSQL(r.db, r.driver, `INSERT INTO saved_filters (owner_user_id, id, name, route, workspace_id, filters_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(owner_user_id, id) DO UPDATE SET name = excluded.name, route = excluded.route, workspace_id = excluded.workspace_id, filters_json = excluded.filters_json, created_at = excluded.created_at, updated_at = excluded.updated_at`, owner, filter.ID, filter.Name, filter.Route, filter.WorkspaceID, filtersJSON, formatTime(filter.CreatedAt), formatTime(filter.UpdatedAt))
	return err
}

func (r *SQLiteNavigationRepository) RestoreRecent(item models.RecentItem) error {
	owner := normalizeNavigationOwner(item.OwnerUserID)
	if item.ItemID == "" {
		return errors.New("recent item snapshot id is required")
	}
	_, err := execSQL(r.db, r.driver, `INSERT INTO recent_items (owner_user_id, item_id, workspace_id, title, subtitle, route, opened_at) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(owner_user_id, item_id) DO UPDATE SET workspace_id = excluded.workspace_id, title = excluded.title, subtitle = excluded.subtitle, route = excluded.route, opened_at = excluded.opened_at`, owner, item.ItemID, item.WorkspaceID, item.Title, item.Subtitle, item.Route, formatTime(item.OpenedAt))
	return err
}

func normalizeNavigationOwner(owner string) string {
	if owner == "" {
		return navigation.LocalOwner
	}
	return owner
}

func navigationID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
}

func (r *SQLiteAISettingsRepository) Load() (ai.Settings, error) {
	var raw string
	err := queryRowSQL(r.db, r.driver, `SELECT settings_json FROM ai_settings WHERE id = 'default'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return ai.Settings{}, nil
	}
	if err != nil {
		return ai.Settings{}, err
	}
	var settings ai.Settings
	return settings, json.Unmarshal([]byte(raw), &settings)
}

func (r *SQLiteAISettingsRepository) Save(settings ai.Settings) (ai.Settings, error) {
	raw, err := encodeJSON(settings)
	if err != nil {
		return ai.Settings{}, err
	}
	_, err = execSQL(r.db, r.driver, `INSERT INTO ai_settings (id, settings_json, updated_at) VALUES ('default', ?, ?) ON CONFLICT(id) DO UPDATE SET settings_json = excluded.settings_json, updated_at = excluded.updated_at`, raw, formatTime(time.Now().UTC()))
	return settings, err
}

func (r *SQLiteImportStatusRepository) ImportCompleted(source string) (bool, error) {
	var value string
	err := queryRowSQL(r.db, r.driver, `SELECT source_name FROM import_status WHERE source_name = ?`, source).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *SQLiteImportStatusRepository) MarkImportCompleted(source string, completedAt time.Time) error {
	_, err := execSQL(r.db, r.driver, `INSERT INTO import_status (source_name, completed_at) VALUES (?, ?) ON CONFLICT(source_name) DO UPDATE SET completed_at = excluded.completed_at`, source, formatTime(completedAt))
	return err
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

func ImportLegacyFiles(paths system.Paths, git *appgit.GitAdapter, state *AppOwnedState) error {
	if state == nil || state.ImportStatus == nil {
		return nil
	}
	if err := importLegacyWorkspacesAtomic(paths.RegistryFile, git, state); err != nil {
		return err
	}
	if err := importLegacyItemsAtomic(paths.PlanIndexFile, state, false); err != nil {
		return err
	}
	if err := importLegacyAuditAtomic(paths.AuditLogFile, state); err != nil {
		return err
	}
	if err := importLegacyNavigationAtomic(paths, state); err != nil {
		return err
	}
	if err := importLegacyAISettingsAtomic(paths.AISettingsFile, state); err != nil {
		return err
	}
	if err := importLegacyCanvasAtomic(paths.CanvasFile, state); err != nil {
		return err
	}
	return importLegacySessionRecordsAtomic(paths.AISessionRecordsFile, state)
}

// RepairLegacyItemImport is an explicit operator-triggered repair seam for
// installations affected by the former branch-collapsing importer. It never
// runs automatically for a source already marked complete.
func RepairLegacyItemImport(path string, state *AppOwnedState) error {
	return importLegacyItemsAtomic(path, state, true)
}

func importLegacyAISettingsAtomic(path string, state *AppOwnedState) error {
	repository, ok := state.AISettings.(*SQLiteAISettingsRepository)
	if !ok {
		return errors.New("legacy AI settings import requires SQL repositories")
	}
	status := state.ImportStatus.(*SQLiteImportStatusRepository)
	done, err := status.ImportCompleted("ai-settings.yaml")
	if err != nil || done {
		return err
	}
	settings, err := ai.NewSettingsRepository(path).Load()
	if err != nil {
		return ignoreMissing(path, err)
	}
	raw, err := encodeJSON(settings)
	if err != nil {
		return err
	}
	tx, err := repository.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if done, err := acquireImportTx(tx, repository.driver, "ai-settings.yaml", false); err != nil || done {
		return err
	}
	if _, err := execTx(tx, repository.driver, `INSERT INTO ai_settings (id, settings_json, updated_at) VALUES ('default', ?, ?) ON CONFLICT(id) DO UPDATE SET settings_json = excluded.settings_json, updated_at = excluded.updated_at`, raw, formatTime(time.Now().UTC())); err != nil {
		return err
	}
	return markImportCompletedTx(tx, repository.driver, "ai-settings.yaml")
}

func importLegacyCanvasAtomic(path string, state *AppOwnedState) error {
	repository, ok := state.Canvas.(*SQLiteCanvasRepository)
	if !ok {
		return errors.New("legacy Canvas import requires SQL repositories")
	}
	status := state.ImportStatus.(*SQLiteImportStatusRepository)
	done, err := status.ImportCompleted("canvases.yaml")
	if err != nil || done {
		return err
	}
	snapshot, err := canvas.NewFileRepository(path).Snapshot()
	if err != nil {
		return ignoreMissing(path, err)
	}
	if err := canvas.ValidateSnapshot(snapshot); err != nil {
		return err
	}
	tx, err := repository.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if done, err := acquireImportTx(tx, repository.driver, "canvases.yaml", false); err != nil || done {
		return err
	}
	if err := replaceCanvasSnapshotTx(tx, repository.driver, snapshot); err != nil {
		return err
	}
	return markImportCompletedTx(tx, repository.driver, "canvases.yaml")
}

func importLegacySessionRecordsAtomic(path string, state *AppOwnedState) error {
	repository, ok := state.SessionRecords.(*SQLiteSessionRecordRepository)
	if !ok {
		return errors.New("legacy session-record import requires SQL repositories")
	}
	status := state.ImportStatus.(*SQLiteImportStatusRepository)
	done, err := status.ImportCompleted("ai-session-records.yaml")
	if err != nil || done {
		return err
	}
	records, err := ai.NewFileSessionRecordRepository(path).Snapshot()
	if err != nil {
		return ignoreMissing(path, err)
	}
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
	tx, err := repository.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if done, err := acquireImportTx(tx, repository.driver, "ai-session-records.yaml", false); err != nil || done {
		return err
	}
	if err := replaceSessionRecordsTx(tx, repository.driver, records); err != nil {
		return err
	}
	return markImportCompletedTx(tx, repository.driver, "ai-session-records.yaml")
}

func importLegacyWorkspacesAtomic(path string, git *appgit.GitAdapter, state *AppOwnedState) error {
	repository, ok := state.Workspaces.(*SQLiteWorkspaceRepository)
	if !ok {
		return errors.New("legacy workspace import requires SQL repositories")
	}
	status := state.ImportStatus.(*SQLiteImportStatusRepository)
	done, err := status.ImportCompleted("workspaces.yaml")
	if err != nil || done {
		return err
	}
	workspaces, err := registry.New(path, git).List()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return ignoreMissing(path, err)
	}
	tx, err := repository.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if done, err := acquireImportTx(tx, repository.driver, "workspaces.yaml", false); err != nil || done {
		return err
	}
	for _, workspace := range workspaces {
		workspace = normalizeWorkspaceForSQL(workspace)
		workspaceJSON, err := encodeJSON(workspace)
		if err != nil {
			return err
		}
		sourcesJSON, err := encodeJSON(workspace.Sources)
		if err != nil {
			return err
		}
		runtimeJSON, err := encodeJSON(workspace.Runtime)
		if err != nil {
			return err
		}
		if _, err := execTx(tx, repository.driver, `INSERT INTO workspaces (id, name, path_label, baseline_branch, registration_mode, remote_url, clone_path_managed, last_selected_branch, sources_json, runtime_json, workspace_json, created_at, last_scanned_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET name = excluded.name, path_label = excluded.path_label, baseline_branch = excluded.baseline_branch, registration_mode = excluded.registration_mode, remote_url = excluded.remote_url, clone_path_managed = excluded.clone_path_managed, last_selected_branch = excluded.last_selected_branch, sources_json = excluded.sources_json, runtime_json = excluded.runtime_json, workspace_json = excluded.workspace_json, created_at = excluded.created_at, last_scanned_at = excluded.last_scanned_at`, workspace.ID, workspace.Name, workspacePathLabel(workspace), workspace.BaselineBranch, workspace.RegistrationMode, workspace.RemoteURL, databaseBoolValue(repository.driver, workspace.ClonePathManaged), workspace.LastSelectedBranch, sourcesJSON, runtimeJSON, workspaceJSON, formatTime(workspace.CreatedAt), formatTime(workspace.LastScannedAt)); err != nil {
			return err
		}
	}
	return markImportCompletedTx(tx, repository.driver, "workspaces.yaml")
}

func importLegacyAuditAtomic(path string, state *AppOwnedState) error {
	repository, ok := state.Audit.(*SQLiteAuditRepository)
	if !ok {
		return errors.New("legacy audit import requires SQL repositories")
	}
	status := state.ImportStatus.(*SQLiteImportStatusRepository)
	done, err := status.ImportCompleted("audit-log.jsonl")
	if err != nil || done {
		return err
	}
	events, err := audit.New(path).Recent(0)
	if err != nil {
		return ignoreMissing(path, err)
	}
	tx, err := repository.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if done, err := acquireImportTx(tx, repository.driver, "audit-log.jsonl", false); err != nil || done {
		return err
	}
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		pathsJSON, err := encodeJSON(event.Paths)
		if err != nil {
			return err
		}
		if _, err := execTx(tx, repository.driver, `INSERT INTO audit_events (id, owner_user_id, actor_user_id, workspace_id, item_id, operation, status, message, paths_json, duration_ms, error, event_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`, event.ID, event.OwnerUserID, event.ActorUserID, event.WorkspaceID, event.ItemID, event.Operation, event.Status, event.Message, pathsJSON, event.DurationMS, event.Error, formatTime(event.Time)); err != nil {
			return err
		}
	}
	return markImportCompletedTx(tx, repository.driver, "audit-log.jsonl")
}

func importLegacyNavigationAtomic(paths system.Paths, state *AppOwnedState) error {
	repository, ok := state.Navigation.(*SQLiteNavigationRepository)
	if !ok {
		return errors.New("legacy navigation import requires SQL repositories")
	}
	status := state.ImportStatus.(*SQLiteImportStatusRepository)
	done, err := status.ImportCompleted("navigation")
	if err != nil || done {
		return err
	}
	legacy := navigation.New(paths.SavedFiltersFile, paths.RecentItemsFile)
	filters, err := legacy.Filters()
	if err != nil {
		return err
	}
	recents, err := legacy.Recents(0)
	if err != nil {
		return err
	}
	tx, err := repository.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if done, err := acquireImportTx(tx, repository.driver, "navigation", false); err != nil || done {
		return err
	}
	for _, filter := range filters {
		owner := normalizeNavigationOwner(filter.OwnerUserID)
		filtersJSON, err := encodeJSON(filter.Filters)
		if err != nil {
			return err
		}
		if _, err := execTx(tx, repository.driver, `INSERT INTO saved_filters (owner_user_id, id, name, route, workspace_id, filters_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(owner_user_id, id) DO UPDATE SET name = excluded.name, route = excluded.route, workspace_id = excluded.workspace_id, filters_json = excluded.filters_json, created_at = excluded.created_at, updated_at = excluded.updated_at`, owner, filter.ID, filter.Name, filter.Route, filter.WorkspaceID, filtersJSON, formatTime(filter.CreatedAt), formatTime(filter.UpdatedAt)); err != nil {
			return err
		}
	}
	for _, recent := range recents {
		owner := normalizeNavigationOwner(recent.OwnerUserID)
		if _, err := execTx(tx, repository.driver, `INSERT INTO recent_items (owner_user_id, item_id, workspace_id, title, subtitle, route, opened_at) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(owner_user_id, item_id) DO UPDATE SET workspace_id = excluded.workspace_id, title = excluded.title, subtitle = excluded.subtitle, route = excluded.route, opened_at = excluded.opened_at`, owner, recent.ItemID, recent.WorkspaceID, recent.Title, recent.Subtitle, recent.Route, formatTime(recent.OpenedAt)); err != nil {
			return err
		}
	}
	return markImportCompletedTx(tx, repository.driver, "navigation")
}

func markImportCompletedTx(tx *sql.Tx, driver, source string) error {
	if _, err := execTx(tx, driver, `INSERT INTO import_status (source_name, completed_at) VALUES (?, ?) ON CONFLICT(source_name) DO UPDATE SET completed_at = excluded.completed_at`, source, formatTime(time.Now().UTC())); err != nil {
		return err
	}
	if _, err := execTx(tx, driver, `DELETE FROM import_locks WHERE source_name = ?`, source); err != nil {
		return err
	}
	return tx.Commit()
}

func acquireImportTx(tx *sql.Tx, driver, source string, repair bool) (bool, error) {
	if _, err := execTx(tx, driver, `INSERT INTO import_locks (source_name) VALUES (?) ON CONFLICT(source_name) DO NOTHING`, source); err != nil {
		return false, err
	}
	if repair {
		return false, nil
	}
	var count int
	if err := tx.QueryRow(rebindSQL(driver, `SELECT COUNT(*) FROM import_status WHERE source_name = ?`), source).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func importLegacyItemsAtomic(path string, state *AppOwnedState, repair bool) error {
	repository, ok := state.Items.(*SQLiteItemRepository)
	if !ok {
		return errors.New("legacy item import requires a SQL item repository")
	}
	status, ok := state.ImportStatus.(*SQLiteImportStatusRepository)
	if !ok {
		return errors.New("legacy item import requires SQL import status")
	}
	done, err := status.ImportCompleted("item-index.yaml")
	if err != nil {
		return err
	}
	if done && !repair {
		return nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return status.MarkImportCompleted("item-index.yaml", time.Now().UTC())
	}
	if err != nil {
		return err
	}
	var legacy fileIndexState
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return err
	}
	for _, item := range legacy.Items {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.WorkspaceID) == "" || strings.TrimSpace(item.Branch) == "" {
			return errors.New("legacy item snapshot contains incomplete identity")
		}
	}
	tx, err := repository.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if done, err := acquireImportTx(tx, repository.driver, "item-index.yaml", repair); err != nil || done {
		return err
	}
	if repair {
		if _, err := execTx(tx, repository.driver, `DELETE FROM import_status WHERE source_name = ?`, "item-index.yaml"); err != nil {
			return err
		}
	}
	workspaces := map[string]bool{}
	for _, item := range legacy.Items {
		workspaces[item.WorkspaceID] = true
	}
	for workspaceID := range legacy.BranchScans {
		workspaces[workspaceID] = true
	}
	for workspaceID := range workspaces {
		for _, table := range []string{"indexed_items", "branch_scans", "scan_warnings"} {
			if _, err := execTx(tx, repository.driver, `DELETE FROM `+table+` WHERE workspace_id = ?`, workspaceID); err != nil {
				return err
			}
		}
	}
	if err := insertItemsRaw(tx, repository.driver, legacy.Items); err != nil {
		return err
	}
	warningKeys := map[string]bool{}
	for workspaceID, branches := range legacy.BranchScans {
		for branch, metadata := range branches {
			metadata.WorkspaceID, metadata.Branch = workspaceID, branch
			if err := insertBranchScan(tx, repository.driver, metadata); err != nil {
				return err
			}
			for _, warning := range metadata.Warnings {
				warningKeys[workspaceID+"\x00"+branch+"\x00"+warning.ItemPath+"\x00"+warning.Message] = true
				if _, err := execTx(tx, repository.driver, `INSERT INTO scan_warnings (workspace_id, branch, item_path, code, message) VALUES (?, ?, ?, ?, ?)`, workspaceID, branch, warning.ItemPath, "", warning.Message); err != nil {
					return err
				}
			}
		}
	}
	for _, warning := range legacy.Warnings {
		identity := strings.SplitN(warning.ItemPath, ":", 3)
		if len(identity) != 3 || !workspaces[identity[0]] {
			continue
		}
		if warningKeys[identity[0]+"\x00"+identity[1]+"\x00"+identity[2]+"\x00"+warning.Message] {
			continue
		}
		if _, err := execTx(tx, repository.driver, `INSERT INTO scan_warnings (workspace_id, branch, item_path, code, message) VALUES (?, ?, ?, ?, ?)`, identity[0], identity[1], identity[2], "", warning.Message); err != nil {
			return err
		}
	}
	return markImportCompletedTx(tx, repository.driver, "item-index.yaml")
}

func insertItemsRaw(tx *sql.Tx, driverName string, items []models.ItemDetail) error {
	for _, item := range items {
		raw, err := encodeJSON(item)
		if err != nil {
			return err
		}
		if _, err := execTx(tx, driverName, `INSERT INTO indexed_items (id, workspace_id, branch, scope, identifier, title, status, item_path, source_mode, editable, metadata_json, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, item.WorkspaceID, item.Branch, item.Scope, item.Identifier, item.Title, item.Status, item.ItemPath, item.SourceMode, databaseBoolValue(driverName, item.Editable), raw, formatTime(item.UpdatedAt)); err != nil {
			return err
		}
	}
	return nil
}

func ignoreMissing(path string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		return nil
	}
	return err
}

func encodeJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	return string(data), err
}

func formatTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func databaseBoolValue(driver string, value bool) any {
	if driver == StorageDriverPostgres {
		return value
	}
	return boolInt(value)
}

func normalizeWorkspaceForSQL(workspace models.WorkspaceConfig) models.WorkspaceConfig {
	if workspace.Sources == nil {
		workspace.Sources = []string{}
	}
	return workspace
}

func workspacePathLabel(workspace models.WorkspaceConfig) string {
	if workspace.LocalRootLabel != "" {
		return workspace.LocalRootLabel
	}
	return workspace.Path
}

func sameWorkspacePath(left, right string) bool {
	return filepath.Clean(strings.TrimSpace(left)) == filepath.Clean(strings.TrimSpace(right))
}

func summaryMatchesText(item models.ItemSummary, text string) bool {
	haystack := strings.ToLower(strings.Join([]string{
		item.Title, item.Identifier, item.Scope, item.Description, item.Author, strings.Join(item.Tags, " "),
	}, " "))
	return strings.Contains(haystack, text)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
