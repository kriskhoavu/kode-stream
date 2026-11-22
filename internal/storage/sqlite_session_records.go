package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"kode-stream/internal/ai"
)

type SQLiteSessionRecordRepository struct {
	db     *sql.DB
	driver string
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
	return r.ListContext(context.Background(), workspaceID, branch)
}

func (r *SQLiteSessionRecordRepository) ListContext(ctx context.Context, workspaceID, branch string) ([]ai.SessionRecord, error) {
	query := sessionRecordSelect + ` WHERE (? = '' OR workspace_id = ?) AND (? = '' OR requested_branch = ?) ORDER BY started_at DESC, id`
	rows, err := r.db.QueryContext(ctx, rebindSQL(r.driver, query), workspaceID, workspaceID, branch, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ai.SessionRecord{}
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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

func (r *SQLiteSessionRecordRepository) Delete(id string) (bool, error) {
	result, err := execSQL(r.db, r.driver, `DELETE FROM ai_session_records WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
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
	if err := replaceSessionRecordsTx(tx, r.driver, records); err != nil {
		return err
	}
	return tx.Commit()
}

func replaceSessionRecordsTx(tx *sql.Tx, driver string, records []ai.SessionRecord) error {
	if _, err := execTx(tx, driver, `DELETE FROM ai_session_records`); err != nil {
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
		if _, err := execTx(tx, driver, `INSERT INTO ai_session_records (id, workspace_id, plan_ref_json, provider, intent, requested_branch, observed_commit, idempotency_key, state, started_at, ended_at, exit_code, last_known_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, record.ID, record.WorkspaceID, planJSON, record.Provider, record.Intent, record.RequestedBranch, record.ObservedCommit, record.IdempotencyKey, record.State, formatTime(record.StartedAt), formatTime(record.EndedAt), exitCode, formatTime(record.LastKnownAt)); err != nil {
			return err
		}
	}
	return nil
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
