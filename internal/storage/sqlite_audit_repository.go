package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"kode-stream/internal/audit"
	"kode-stream/internal/common/models"
)

type SQLiteAuditRepository struct {
	db     *sql.DB
	driver string
	now    func() time.Time
	mu     sync.Mutex
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
