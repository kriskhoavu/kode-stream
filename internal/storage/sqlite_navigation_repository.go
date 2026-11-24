package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/navigation"
)

type SQLiteNavigationRepository struct {
	db     *sql.DB
	driver string
	now    func() time.Time
	mu     sync.Mutex
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
