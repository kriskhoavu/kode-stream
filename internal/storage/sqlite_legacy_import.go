package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"kode-stream/internal/ai"
	"kode-stream/internal/audit"
	"kode-stream/internal/canvas"
	"kode-stream/internal/common/models"
	appgit "kode-stream/internal/git"
	"kode-stream/internal/navigation"
	"kode-stream/internal/system"
	"kode-stream/internal/workspace/registry"
)

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
	legacy, found, err := readLegacyItemIndex(path)
	if err != nil {
		return err
	}
	if !found {
		return status.MarkImportCompleted("item-index.yaml", time.Now().UTC())
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
	workspaces, err := clearLegacyWorkspaces(tx, repository.driver, legacy)
	if err != nil {
		return err
	}
	if err := insertItemsRaw(tx, repository.driver, legacy.Items); err != nil {
		return err
	}
	if err := insertLegacyScans(tx, repository.driver, legacy, workspaces); err != nil {
		return err
	}
	return markImportCompletedTx(tx, repository.driver, "item-index.yaml")
}

// readLegacyItemIndex reads the legacy item index. found is false when the
// file does not exist. Every item must name its id, workspace and branch.
func readLegacyItemIndex(path string) (fileIndexState, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileIndexState{}, false, nil
	}
	if err != nil {
		return fileIndexState{}, false, err
	}
	var legacy fileIndexState
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return fileIndexState{}, false, err
	}
	for _, item := range legacy.Items {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.WorkspaceID) == "" || strings.TrimSpace(item.Branch) == "" {
			return fileIndexState{}, false, errors.New("legacy item snapshot contains incomplete identity")
		}
	}
	return legacy, true, nil
}

// clearLegacyWorkspaces deletes the indexed rows of every workspace the
// legacy index names, and returns those workspaces.
func clearLegacyWorkspaces(tx *sql.Tx, driver string, legacy fileIndexState) (map[string]bool, error) {
	workspaces := map[string]bool{}
	for _, item := range legacy.Items {
		workspaces[item.WorkspaceID] = true
	}
	for workspaceID := range legacy.BranchScans {
		workspaces[workspaceID] = true
	}
	for workspaceID := range workspaces {
		for _, table := range []string{"indexed_items", "branch_scans", "scan_warnings"} {
			if _, err := execTx(tx, driver, `DELETE FROM `+table+` WHERE workspace_id = ?`, workspaceID); err != nil {
				return nil, err
			}
		}
	}
	return workspaces, nil
}

const insertScanWarningSQL = `INSERT INTO scan_warnings (workspace_id, branch, item_path, code, message) VALUES (?, ?, ?, ?, ?)`

// insertLegacyScans inserts each branch scan with its warnings, then the
// index-wide warnings of a known workspace that no scan already holds.
func insertLegacyScans(tx *sql.Tx, driver string, legacy fileIndexState, workspaces map[string]bool) error {
	warningKeys := map[string]bool{}
	for workspaceID, branches := range legacy.BranchScans {
		for branch, metadata := range branches {
			metadata.WorkspaceID, metadata.Branch = workspaceID, branch
			if err := insertBranchScan(tx, driver, metadata); err != nil {
				return err
			}
			for _, warning := range metadata.Warnings {
				warningKeys[workspaceID+"\x00"+branch+"\x00"+warning.ItemPath+"\x00"+warning.Message] = true
				if _, err := execTx(tx, driver, insertScanWarningSQL, workspaceID, branch, warning.ItemPath, "", warning.Message); err != nil {
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
		if _, err := execTx(tx, driver, insertScanWarningSQL, identity[0], identity[1], identity[2], "", warning.Message); err != nil {
			return err
		}
	}
	return nil
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
