package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"kode-stream/internal/harness/domain"
)

func (r *SQLHarnessRepository) InsertTask(ctx context.Context, task domain.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	_, err = r.exec(ctx, `INSERT INTO harness_tasks (id, workspace_id, source_kind, source_key, state, score, pinned, run_id, revision, data_json, discovered_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.WorkspaceID, string(task.Source.Kind), task.Source.Key, string(task.State), task.Score, boolInt(task.Pinned), task.RunID, task.Revision, string(data), harnessTime(task.DiscoveredAt), harnessTime(task.UpdatedAt))
	if isUniqueViolation(err) {
		return domain.ErrDuplicate
	}
	return err
}

const taskColumns = `state, revision, data_json`

func scanTask(row interface{ Scan(...any) error }) (domain.Task, error) {
	var state, data string
	var revision int64
	if err := row.Scan(&state, &revision, &data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, domain.ErrNotFound
		}
		return domain.Task{}, err
	}
	var task domain.Task
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		return domain.Task{}, err
	}
	task.State = domain.TaskState(state)
	task.Revision = revision
	if task.Source.Labels == nil {
		task.Source.Labels = []string{}
	}
	if task.ScoreReasons == nil {
		task.ScoreReasons = []string{}
	}
	if task.DependsOn == nil {
		task.DependsOn = []string{}
	}
	return task, nil
}

func (r *SQLHarnessRepository) GetTask(ctx context.Context, id string) (domain.Task, error) {
	return scanTask(r.queryRow(ctx, `SELECT `+taskColumns+` FROM harness_tasks WHERE id = ?`, id))
}

func (r *SQLHarnessRepository) FindTaskBySource(ctx context.Context, kind domain.SourceKind, key string) (domain.Task, bool, error) {
	task, err := scanTask(r.queryRow(ctx, `SELECT `+taskColumns+` FROM harness_tasks WHERE source_kind = ? AND source_key = ?`, string(kind), key))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Task{}, false, nil
	}
	return task, err == nil, err
}

func (r *SQLHarnessRepository) ListTasks(ctx context.Context, filter domain.TaskFilter) ([]domain.Task, error) {
	query := `SELECT ` + taskColumns + ` FROM harness_tasks WHERE 1 = 1`
	args := []any{}
	if filter.WorkspaceID != "" {
		query += ` AND workspace_id = ?`
		args = append(args, filter.WorkspaceID)
	}
	if len(filter.States) > 0 {
		query += ` AND state IN ` + inClause(len(filter.States))
		args = append(args, stringArgs(filter.States)...)
	}
	query += fmt.Sprintf(` ORDER BY pinned DESC, score DESC, discovered_at ASC, id ASC LIMIT %d`, limitOr(filter.Limit, 1000))
	rows, err := r.query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []domain.Task{}
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *SQLHarnessRepository) SaveTask(ctx context.Context, task domain.Task, expect domain.Expect) (domain.Task, error) {
	task.Revision = expect.Revision + 1
	data, err := json.Marshal(task)
	if err != nil {
		return task, err
	}
	err = conditional(r.exec(ctx, `UPDATE harness_tasks SET state = ?, score = ?, pinned = ?, run_id = ?, revision = ?, data_json = ?, updated_at = ? WHERE id = ? AND state = ? AND revision = ?`,
		string(task.State), task.Score, boolInt(task.Pinned), task.RunID, task.Revision, string(data), harnessTime(task.UpdatedAt), task.ID, expect.State, expect.Revision))
	return task, err
}

func (r *SQLHarnessRepository) DeleteTasks(ctx context.Context, filter domain.TaskFilter) ([]domain.Task, error) {
	removed := []domain.Task{}
	for {
		batch, err := r.ListTasks(ctx, domain.TaskFilter{WorkspaceID: filter.WorkspaceID, States: filter.States, Limit: 1000})
		if err != nil {
			return removed, err
		}
		if len(batch) == 0 {
			return removed, nil
		}
		ids := make([]string, 0, len(batch))
		for _, task := range batch {
			ids = append(ids, task.ID)
		}
		if _, err := r.exec(ctx, `DELETE FROM harness_tasks WHERE id IN `+inClause(len(ids)), stringArgs(ids)...); err != nil {
			return removed, err
		}
		removed = append(removed, batch...)
		if len(batch) < 1000 {
			return removed, nil
		}
	}
}

func (r *SQLHarnessRepository) DeleteTaskHistory(ctx context.Context, taskID string) error {
	runs := `SELECT id FROM harness_runs WHERE task_id = ?`
	statements := []string{
		`DELETE FROM harness_agent_runs WHERE run_id IN (` + runs + `)`,
		`DELETE FROM harness_stages WHERE run_id IN (` + runs + `)`,
		`DELETE FROM harness_approvals WHERE task_id = ?`,
		`DELETE FROM harness_runs WHERE task_id = ?`,
		`DELETE FROM harness_tasks WHERE id = ?`,
	}
	for _, statement := range statements {
		if _, err := r.exec(ctx, statement, taskID); err != nil {
			return err
		}
	}
	return nil
}
