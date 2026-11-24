package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"kode-stream/internal/harness/domain"
)

func (r *SQLHarnessRepository) InsertApproval(ctx context.Context, approval domain.Approval) error {
	data, err := json.Marshal(approvalRecord{Approval: approval, IdempotencyKey: approval.IdempotencyKey, RequestID: approval.RequestID})
	if err != nil {
		return err
	}
	_, err = r.exec(ctx, `INSERT INTO harness_approvals (id, run_id, task_id, stage_id, gate, state, revision, idempotency_key, requested_at, data_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		approval.ID, approval.RunID, approval.TaskID, approval.StageID, string(approval.Gate), string(approval.State), approval.Revision, approval.IdempotencyKey, harnessTime(approval.RequestedAt), string(data))
	if isUniqueViolation(err) {
		return domain.ErrDuplicate
	}
	return err
}

type approvalRecord struct {
	domain.Approval
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
	RequestID      string `json:"requestId,omitempty"`
}

func scanApproval(row interface{ Scan(...any) error }) (domain.Approval, error) {
	var state, data string
	var revision int64
	if err := row.Scan(&state, &revision, &data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Approval{}, domain.ErrNotFound
		}
		return domain.Approval{}, err
	}
	var record approvalRecord
	if err := json.Unmarshal([]byte(data), &record); err != nil {
		return domain.Approval{}, err
	}
	approval := record.Approval
	approval.IdempotencyKey = record.IdempotencyKey
	approval.RequestID = record.RequestID
	approval.State = domain.ApprovalState(state)
	approval.Revision = revision
	return approval, nil
}

func (r *SQLHarnessRepository) GetApproval(ctx context.Context, id string) (domain.Approval, error) {
	return scanApproval(r.queryRow(ctx, `SELECT state, revision, data_json FROM harness_approvals WHERE id = ?`, id))
}

func (r *SQLHarnessRepository) FindApprovalByIdempotencyKey(ctx context.Context, key string) (domain.Approval, bool, error) {
	if strings.TrimSpace(key) == "" {
		return domain.Approval{}, false, nil
	}
	approval, err := scanApproval(r.queryRow(ctx, `SELECT state, revision, data_json FROM harness_approvals WHERE idempotency_key = ?`, key))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Approval{}, false, nil
	}
	return approval, err == nil, err
}

func (r *SQLHarnessRepository) ListApprovals(ctx context.Context, filter domain.ApprovalFilter) ([]domain.Approval, error) {
	query := `SELECT state, revision, data_json FROM harness_approvals WHERE 1 = 1`
	args := []any{}
	if filter.RunID != "" {
		query += ` AND run_id = ?`
		args = append(args, filter.RunID)
	}
	if filter.TaskID != "" {
		query += ` AND task_id = ?`
		args = append(args, filter.TaskID)
	}
	if filter.StageID != "" {
		query += ` AND stage_id = ?`
		args = append(args, filter.StageID)
	}
	if len(filter.States) > 0 {
		query += ` AND state IN ` + inClause(len(filter.States))
		args = append(args, stringArgs(filter.States)...)
	}
	query += fmt.Sprintf(` ORDER BY requested_at DESC, id DESC LIMIT %d`, limitOr(filter.Limit, 200))
	rows, err := r.query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	approvals := []domain.Approval{}
	for rows.Next() {
		approval, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, approval)
	}
	return approvals, rows.Err()
}

func (r *SQLHarnessRepository) SaveApproval(ctx context.Context, approval domain.Approval, expect domain.Expect) (domain.Approval, error) {
	approval.Revision = expect.Revision + 1
	data, err := json.Marshal(approvalRecord{Approval: approval, IdempotencyKey: approval.IdempotencyKey, RequestID: approval.RequestID})
	if err != nil {
		return approval, err
	}
	err = conditional(r.exec(ctx, `UPDATE harness_approvals SET state = ?, revision = ?, idempotency_key = ?, data_json = ? WHERE id = ? AND state = ? AND revision = ?`,
		string(approval.State), approval.Revision, approval.IdempotencyKey, string(data), approval.ID, expect.State, expect.Revision))
	return approval, err
}
