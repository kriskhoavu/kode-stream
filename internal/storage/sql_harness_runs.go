package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"kode-stream/internal/harness/domain"
)

func (r *SQLHarnessRepository) InsertRun(ctx context.Context, run domain.Run) error {
	data, err := json.Marshal(run)
	if err != nil {
		return err
	}
	_, err = r.exec(ctx, `INSERT INTO harness_runs (id, task_id, workspace_id, state, revision, data_json, started_at, finished_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.TaskID, run.WorkspaceID, string(run.State), run.Revision, string(data), harnessTime(run.StartedAt), harnessTimePtr(run.FinishedAt))
	return err
}

func scanRun(row interface{ Scan(...any) error }) (domain.Run, error) {
	var state, data string
	var revision int64
	if err := row.Scan(&state, &revision, &data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Run{}, domain.ErrNotFound
		}
		return domain.Run{}, err
	}
	var run domain.Run
	if err := json.Unmarshal([]byte(data), &run); err != nil {
		return domain.Run{}, err
	}
	run.State = domain.RunState(state)
	run.Revision = revision
	return run, nil
}

func (r *SQLHarnessRepository) GetRun(ctx context.Context, id string) (domain.Run, error) {
	return scanRun(r.queryRow(ctx, `SELECT state, revision, data_json FROM harness_runs WHERE id = ?`, id))
}

func (r *SQLHarnessRepository) ListRuns(ctx context.Context, filter domain.RunFilter) ([]domain.Run, error) {
	query := `SELECT state, revision, data_json FROM harness_runs WHERE 1 = 1`
	args := []any{}
	if filter.TaskID != "" {
		query += ` AND task_id = ?`
		args = append(args, filter.TaskID)
	}
	if filter.WorkspaceID != "" {
		query += ` AND workspace_id = ?`
		args = append(args, filter.WorkspaceID)
	}
	if len(filter.States) > 0 {
		query += ` AND state IN ` + inClause(len(filter.States))
		args = append(args, stringArgs(filter.States)...)
	}
	query += fmt.Sprintf(` ORDER BY started_at DESC, id DESC LIMIT %d`, limitOr(filter.Limit, 200))
	rows, err := r.query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []domain.Run{}
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *SQLHarnessRepository) SaveRun(ctx context.Context, run domain.Run, expect domain.Expect) (domain.Run, error) {
	run.Revision = expect.Revision + 1
	data, err := json.Marshal(run)
	if err != nil {
		return run, err
	}
	err = conditional(r.exec(ctx, `UPDATE harness_runs SET state = ?, revision = ?, data_json = ?, finished_at = ? WHERE id = ? AND state = ? AND revision = ?`,
		string(run.State), run.Revision, string(data), harnessTimePtr(run.FinishedAt), run.ID, expect.State, expect.Revision))
	return run, err
}

func (r *SQLHarnessRepository) InsertStage(ctx context.Context, stage domain.Stage) error {
	data, err := json.Marshal(stage)
	if err != nil {
		return err
	}
	_, err = r.exec(ctx, `INSERT INTO harness_stages (id, run_id, kind, ordinal, state, revision, lease_owner, lease_until, retry_at, data_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		stage.ID, stage.RunID, string(stage.Kind), stage.Ordinal, string(stage.State), stage.Revision, stage.LeaseOwner, harnessTimePtr(stage.LeaseUntil), harnessTimePtr(stage.RetryAt), string(data), harnessTime(time.Now()))
	return err
}

func scanStage(row interface{ Scan(...any) error }) (domain.Stage, error) {
	var state, owner, data string
	var revision int64
	var leaseUntil sql.NullString
	if err := row.Scan(&state, &revision, &owner, &leaseUntil, &data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Stage{}, domain.ErrNotFound
		}
		return domain.Stage{}, err
	}
	var stage domain.Stage
	if err := json.Unmarshal([]byte(data), &stage); err != nil {
		return domain.Stage{}, err
	}
	stage.State = domain.StageState(state)
	stage.Revision = revision
	stage.LeaseOwner = owner
	if leaseUntil.Valid && leaseUntil.String != "" {
		if parsed, err := time.Parse(harnessTimeLayout, leaseUntil.String); err == nil {
			stage.LeaseUntil = &parsed
		}
	}
	if stage.Rounds == nil {
		stage.Rounds = []domain.Round{}
	}
	return stage, nil
}

const stageColumns = `state, revision, lease_owner, lease_until, data_json`

func (r *SQLHarnessRepository) GetStage(ctx context.Context, id string) (domain.Stage, error) {
	return scanStage(r.queryRow(ctx, `SELECT `+stageColumns+` FROM harness_stages WHERE id = ?`, id))
}

func (r *SQLHarnessRepository) ListStages(ctx context.Context, runID string) ([]domain.Stage, error) {
	rows, err := r.query(ctx, `SELECT `+stageColumns+` FROM harness_stages WHERE run_id = ? ORDER BY ordinal ASC, created_at ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stages := []domain.Stage{}
	for rows.Next() {
		stage, err := scanStage(rows)
		if err != nil {
			return nil, err
		}
		stages = append(stages, stage)
	}
	return stages, rows.Err()
}

func (r *SQLHarnessRepository) SaveStage(ctx context.Context, stage domain.Stage, expect domain.Expect) (domain.Stage, error) {
	stage.Revision = expect.Revision + 1
	data, err := json.Marshal(stage)
	if err != nil {
		return stage, err
	}
	err = conditional(r.exec(ctx, `UPDATE harness_stages SET state = ?, revision = ?, lease_owner = ?, lease_until = ?, retry_at = ?, data_json = ? WHERE id = ? AND state = ? AND revision = ?`,
		string(stage.State), stage.Revision, stage.LeaseOwner, harnessTimePtr(stage.LeaseUntil), harnessTimePtr(stage.RetryAt), string(data), stage.ID, expect.State, expect.Revision))
	return stage, err
}

func (r *SQLHarnessRepository) lockSuffix() string {
	if r.driver == StorageDriverPostgres {
		return ` FOR UPDATE SKIP LOCKED`
	}
	return ``
}

func (r *SQLHarnessRepository) ClaimStage(ctx context.Context, kinds []domain.StageKind, claim domain.Claim) (domain.Stage, bool, error) {
	if len(kinds) == 0 {
		return domain.Stage{}, false, nil
	}
	var claimed domain.Stage
	found := false
	err := r.Atomic(ctx, func(repository domain.Repository) error {
		tx := repository.(*SQLHarnessRepository)
		args := append(stringArgs(kinds), string(domain.StagePending), harnessTime(claim.Now))
		stage, err := scanStage(tx.queryRow(ctx, `SELECT `+stageColumns+` FROM harness_stages WHERE kind IN `+inClause(len(kinds))+` AND state = ? AND (retry_at IS NULL OR retry_at <= ?) ORDER BY created_at ASC, id ASC LIMIT 1`+tx.lockSuffix(), args...))
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		expect := domain.Expect{State: string(stage.State), Revision: stage.Revision}
		stage.State = domain.StageRunning
		stage.RetryAt = nil
		stage.LeaseOwner = claim.Owner
		leaseUntil := claim.LeaseUntil
		stage.LeaseUntil = &leaseUntil
		if stage.StartedAt == nil {
			now := claim.Now
			stage.StartedAt = &now
		}
		claimed, err = tx.SaveStage(ctx, stage, expect)
		found = err == nil
		return err
	})
	return claimed, found, err
}
