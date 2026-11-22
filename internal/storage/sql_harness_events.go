package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"kode-stream/internal/harness/domain"
)

type eventRecord struct {
	TaskID     string          `json:"taskId,omitempty"`
	RunID      string          `json:"runId,omitempty"`
	StageID    string          `json:"stageId,omitempty"`
	AgentRunID string          `json:"agentRunId,omitempty"`
	ApprovalID string          `json:"approvalId,omitempty"`
	Actor      *domain.Actor   `json:"actor,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

func (r *SQLHarnessRepository) AppendEvent(ctx context.Context, event domain.Event) (domain.Event, error) {
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}
	if len(event.Payload) > domain.MaxPayloadBytes {
		event.Payload = json.RawMessage(`{"truncated":true}`)
	}
	data, err := json.Marshal(eventRecord{TaskID: event.TaskID, RunID: event.RunID, StageID: event.StageID, AgentRunID: event.AgentRunID, ApprovalID: event.ApprovalID, Actor: event.Actor, Payload: event.Payload})
	if err != nil {
		return event, err
	}
	if r.driver == StorageDriverPostgres {
		err = r.queryRow(ctx, `INSERT INTO harness_events (kind, at, data_json) VALUES (?, ?, ?) RETURNING seq`, string(event.Kind), harnessTime(event.At), string(data)).Scan(&event.Seq)
		return event, err
	}
	result, err := r.exec(ctx, `INSERT INTO harness_events (kind, at, data_json) VALUES (?, ?, ?)`, string(event.Kind), harnessTime(event.At), string(data))
	if err != nil {
		return event, err
	}
	event.Seq, err = result.LastInsertId()
	return event, err
}

func (r *SQLHarnessRepository) EventsAfter(ctx context.Context, after int64, limit int) ([]domain.Event, error) {
	rows, err := r.query(ctx, fmt.Sprintf(`SELECT seq, kind, at, data_json FROM harness_events WHERE seq > ? ORDER BY seq ASC LIMIT %d`, limitOr(limit, 500)), after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []domain.Event{}
	for rows.Next() {
		var event domain.Event
		var kind, at, data string
		if err := rows.Scan(&event.Seq, &kind, &at, &data); err != nil {
			return nil, err
		}
		var record eventRecord
		if err := json.Unmarshal([]byte(data), &record); err != nil {
			return nil, err
		}
		event.Kind = domain.EventKind(kind)
		event.At, _ = time.Parse(harnessTimeLayout, at)
		event.TaskID, event.RunID, event.StageID, event.AgentRunID, event.ApprovalID, event.Actor = record.TaskID, record.RunID, record.StageID, record.AgentRunID, record.ApprovalID, record.Actor
		event.Payload = record.Payload
		if len(event.Payload) == 0 {
			event.Payload = json.RawMessage(`{}`)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *SQLHarnessRepository) LastEventSeq(ctx context.Context) (int64, error) {
	var seq int64
	err := r.queryRow(ctx, `SELECT COALESCE(MAX(seq), 0) FROM harness_events`).Scan(&seq)
	return seq, err
}

func (r *SQLHarnessRepository) EnqueueNotification(ctx context.Context, notification domain.Notification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	_, err = r.exec(ctx, `INSERT INTO harness_outbox (id, event_seq, target, state, attempts, next_at, data_json) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		notification.ID, notification.EventSeq, notification.Target, string(notification.State), notification.Attempts, harnessTime(notification.NextAt), string(data))
	return err
}

func (r *SQLHarnessRepository) DueNotifications(ctx context.Context, now time.Time, limit int) ([]domain.Notification, error) {
	rows, err := r.query(ctx, fmt.Sprintf(`SELECT data_json, state, attempts FROM harness_outbox WHERE state = ? AND next_at <= ? ORDER BY next_at ASC, event_seq ASC LIMIT %d`, limitOr(limit, 20)), string(domain.NotificationPending), harnessTime(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	notifications := []domain.Notification{}
	for rows.Next() {
		var data, state string
		var attempts int
		if err := rows.Scan(&data, &state, &attempts); err != nil {
			return nil, err
		}
		var notification domain.Notification
		if err := json.Unmarshal([]byte(data), &notification); err != nil {
			return nil, err
		}
		notification.State = domain.NotificationState(state)
		notification.Attempts = attempts
		notifications = append(notifications, notification)
	}
	return notifications, rows.Err()
}

func (r *SQLHarnessRepository) SaveNotification(ctx context.Context, notification domain.Notification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	_, err = r.exec(ctx, `UPDATE harness_outbox SET state = ?, attempts = ?, next_at = ?, data_json = ? WHERE id = ?`,
		string(notification.State), notification.Attempts, harnessTime(notification.NextAt), string(data), notification.ID)
	return err
}
