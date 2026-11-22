package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kode-stream/internal/common/models"
	appgit "kode-stream/internal/git"
	"kode-stream/internal/system"
	"kode-stream/internal/workspace/registry"
)

type SQLiteWorkspaceRepository struct {
	db        *sql.DB
	driver    string
	path      string
	validator *registry.Registry
	mu        sync.Mutex
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
	if existing.ClonePathManaged {
		if !sameWorkspacePath(existing.Path, workspace.Path) {
			return models.WorkspaceConfig{}, fmt.Errorf("managed clone path is immutable")
		}
		workspace.ClonePathManaged = existing.ClonePathManaged
		workspace.ManagedCloneRoot = existing.ManagedCloneRoot
		workspace.ManagedCloneID = existing.ManagedCloneID
		workspace.ManagedCloneVerified = existing.ManagedCloneVerified
		workspace.ManagedCloneCleanupPending = existing.ManagedCloneCleanupPending
	}
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

func (r *SQLiteWorkspaceRepository) MarkManagedCloneCleanup(id string) error {
	return r.patch(id, func(workspace *models.WorkspaceConfig) { workspace.ManagedCloneCleanupPending = true })
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

func normalizeWorkspaceForSQL(workspace models.WorkspaceConfig) models.WorkspaceConfig {
	if workspace.Sources == nil {
		workspace.Sources = []models.WorkspaceSource{}
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
