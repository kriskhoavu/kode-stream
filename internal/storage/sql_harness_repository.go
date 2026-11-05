package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"kode-stream/internal/harness/domain"
)

// SQLHarnessRepository is the SQLite and Postgres implementation of the
// harness repository. Each row keeps the columns the queue filters, claims
// and sums on, and the rest of the entity as JSON.
type SQLHarnessRepository struct {
	db     *sql.DB
	q      harnessQuerier
	driver string
	inTx   bool
}

type harnessQuerier interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const harnessTimeLayout = "2006-01-02T15:04:05.000000Z"

func newSQLHarnessRepository(store *SQLStore) *SQLHarnessRepository {
	return &SQLHarnessRepository{db: store.db, q: store.db, driver: store.driver}
}

// OpenSQLiteHarness opens a SQLite database at path with every migration
// applied and returns its harness repository. Tests and tools use it; the
// server gets the repository from OpenAppOwnedState.
func OpenSQLiteHarness(path string) (domain.Repository, func() error, error) {
	store, err := openSQLStore(Config{Driver: StorageDriverSQLite, SQLitePath: path, Migrations: "auto"})
	if err != nil {
		return nil, nil, err
	}
	return newSQLHarnessRepository(store), store.Close, nil
}

func harnessTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(harnessTimeLayout)
}

func harnessTimePtr(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return harnessTime(*value)
}

func (r *SQLHarnessRepository) bind(query string) string { return rebindSQL(r.driver, query) }

func (r *SQLHarnessRepository) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.q.ExecContext(ctx, r.bind(query), args...)
}

func (r *SQLHarnessRepository) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return r.q.QueryContext(ctx, r.bind(query), args...)
}

func (r *SQLHarnessRepository) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return r.q.QueryRowContext(ctx, r.bind(query), args...)
}

func (r *SQLHarnessRepository) Atomic(ctx context.Context, fn func(domain.Repository) error) error {
	if r.inTx {
		return fn(r)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	child := &SQLHarnessRepository{db: r.db, q: tx, driver: r.driver, inTx: true}
	if err := fn(child); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate key")
}

// conditional checks a conditional UPDATE changed exactly one row.
func conditional(result sql.Result, err error) error {
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicate
		}
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrStateMoved
	}
	return nil
}

func inClause(count int) string {
	return "(" + strings.TrimSuffix(strings.Repeat("?,", count), ",") + ")"
}

func stringArgs[S ~string](values []S) []any {
	args := make([]any, len(values))
	for i, value := range values {
		args[i] = string(value)
	}
	return args
}

func limitOr(limit, fallback int) int {
	if limit <= 0 || limit > 1000 {
		return fallback
	}
	return limit
}

// --- tasks ---

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

// --- runs ---

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

// --- stages ---

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

// --- agent runs ---

func (r *SQLHarnessRepository) InsertAgentRun(ctx context.Context, agentRun domain.AgentRun) error {
	data, err := json.Marshal(agentRun)
	if err != nil {
		return err
	}
	_, err = r.exec(ctx, `INSERT INTO harness_agent_runs (id, run_id, stage_id, role, engine, model, state, revision, lease_owner, lease_until, cost_usd, turns, queued_at, finished_at, data_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agentRun.ID, agentRun.RunID, agentRun.StageID, string(agentRun.Role), agentRun.Engine, agentRun.Model, string(agentRun.State), agentRun.Revision, agentRun.LeaseOwner, harnessTimePtr(agentRun.LeaseUntil), agentRun.CostUSD, agentRun.Turns, harnessTime(agentRun.QueuedAt), harnessTimePtr(agentRun.FinishedAt), string(data))
	return err
}

const agentRunColumns = `state, revision, lease_owner, lease_until, data_json`

func scanAgentRun(row interface{ Scan(...any) error }) (domain.AgentRun, error) {
	var state, owner, data string
	var revision int64
	var leaseUntil sql.NullString
	if err := row.Scan(&state, &revision, &owner, &leaseUntil, &data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.AgentRun{}, domain.ErrNotFound
		}
		return domain.AgentRun{}, err
	}
	var agentRun domain.AgentRun
	if err := json.Unmarshal([]byte(data), &agentRun); err != nil {
		return domain.AgentRun{}, err
	}
	agentRun.State = domain.AgentRunState(state)
	agentRun.Revision = revision
	agentRun.LeaseOwner = owner
	if leaseUntil.Valid && leaseUntil.String != "" {
		if parsed, err := time.Parse(harnessTimeLayout, leaseUntil.String); err == nil {
			agentRun.LeaseUntil = &parsed
		}
	}
	return agentRun, nil
}

func (r *SQLHarnessRepository) GetAgentRun(ctx context.Context, id string) (domain.AgentRun, error) {
	return scanAgentRun(r.queryRow(ctx, `SELECT `+agentRunColumns+` FROM harness_agent_runs WHERE id = ?`, id))
}

func (r *SQLHarnessRepository) ListAgentRuns(ctx context.Context, filter domain.AgentRunFilter) ([]domain.AgentRun, error) {
	query := `SELECT ` + agentRunColumns + ` FROM harness_agent_runs WHERE 1 = 1`
	args := []any{}
	if filter.RunID != "" {
		query += ` AND run_id = ?`
		args = append(args, filter.RunID)
	}
	if filter.StageID != "" {
		query += ` AND stage_id = ?`
		args = append(args, filter.StageID)
	}
	if len(filter.States) > 0 {
		query += ` AND state IN ` + inClause(len(filter.States))
		args = append(args, stringArgs(filter.States)...)
	}
	query += fmt.Sprintf(` ORDER BY queued_at ASC, id ASC LIMIT %d`, limitOr(filter.Limit, 500))
	rows, err := r.query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	agentRuns := []domain.AgentRun{}
	for rows.Next() {
		agentRun, err := scanAgentRun(rows)
		if err != nil {
			return nil, err
		}
		agentRuns = append(agentRuns, agentRun)
	}
	return agentRuns, rows.Err()
}

func (r *SQLHarnessRepository) SaveAgentRun(ctx context.Context, agentRun domain.AgentRun, expect domain.Expect) (domain.AgentRun, error) {
	agentRun.Revision = expect.Revision + 1
	data, err := json.Marshal(agentRun)
	if err != nil {
		return agentRun, err
	}
	err = conditional(r.exec(ctx, `UPDATE harness_agent_runs SET state = ?, revision = ?, model = ?, lease_owner = ?, lease_until = ?, cost_usd = ?, turns = ?, finished_at = ?, data_json = ? WHERE id = ? AND state = ? AND revision = ?`,
		string(agentRun.State), agentRun.Revision, agentRun.Model, agentRun.LeaseOwner, harnessTimePtr(agentRun.LeaseUntil), agentRun.CostUSD, agentRun.Turns, harnessTimePtr(agentRun.FinishedAt), string(data), agentRun.ID, expect.State, expect.Revision))
	return agentRun, err
}

func (r *SQLHarnessRepository) ClaimAgentRun(ctx context.Context, roles []domain.Role, claim domain.Claim) (domain.AgentRun, bool, error) {
	if len(roles) == 0 {
		return domain.AgentRun{}, false, nil
	}
	var claimed domain.AgentRun
	found := false
	err := r.Atomic(ctx, func(repository domain.Repository) error {
		tx := repository.(*SQLHarnessRepository)
		args := append(stringArgs(roles), string(domain.AgentQueued))
		agentRun, err := scanAgentRun(tx.queryRow(ctx, `SELECT `+agentRunColumns+` FROM harness_agent_runs WHERE role IN `+inClause(len(roles))+` AND state = ? ORDER BY queued_at ASC, id ASC LIMIT 1`+tx.lockSuffix(), args...))
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		expect := domain.Expect{State: string(agentRun.State), Revision: agentRun.Revision}
		agentRun.State = domain.AgentRunning
		agentRun.LeaseOwner = claim.Owner
		leaseUntil := claim.LeaseUntil
		agentRun.LeaseUntil = &leaseUntil
		now := claim.Now
		agentRun.StartedAt = &now
		agentRun.HeartbeatAt = &now
		claimed, err = tx.SaveAgentRun(ctx, agentRun, expect)
		found = err == nil
		return err
	})
	return claimed, found, err
}

func (r *SQLHarnessRepository) RecoverExpiredLeases(ctx context.Context, now time.Time) ([]domain.AgentRun, []domain.Stage, error) {
	var recoveredRuns []domain.AgentRun
	var recoveredStages []domain.Stage
	err := r.Atomic(ctx, func(repository domain.Repository) error {
		tx := repository.(*SQLHarnessRepository)
		rows, err := tx.query(ctx, `SELECT `+agentRunColumns+` FROM harness_agent_runs WHERE state = ? AND lease_until IS NOT NULL AND lease_until < ?`, string(domain.AgentRunning), harnessTime(now))
		if err != nil {
			return err
		}
		var expired []domain.AgentRun
		for rows.Next() {
			agentRun, err := scanAgentRun(rows)
			if err != nil {
				rows.Close()
				return err
			}
			expired = append(expired, agentRun)
		}
		rows.Close()
		for _, agentRun := range expired {
			expect := domain.Expect{State: string(agentRun.State), Revision: agentRun.Revision}
			agentRun.State = domain.AgentInterrupted
			agentRun.LeaseOwner = ""
			agentRun.LeaseUntil = nil
			agentRun.FailureReason = "lease expired"
			saved, err := tx.SaveAgentRun(ctx, agentRun, expect)
			if err != nil {
				return err
			}
			recoveredRuns = append(recoveredRuns, saved)
		}
		stageRows, err := tx.query(ctx, `SELECT `+stageColumns+` FROM harness_stages WHERE state = ? AND lease_until IS NOT NULL AND lease_until < ?`, string(domain.StageRunning), harnessTime(now))
		if err != nil {
			return err
		}
		var stages []domain.Stage
		for stageRows.Next() {
			stage, err := scanStage(stageRows)
			if err != nil {
				stageRows.Close()
				return err
			}
			stages = append(stages, stage)
		}
		stageRows.Close()
		for _, stage := range stages {
			expect := domain.Expect{State: string(stage.State), Revision: stage.Revision}
			stage.State = domain.StagePending
			stage.LeaseOwner = ""
			stage.LeaseUntil = nil
			saved, err := tx.SaveStage(ctx, stage, expect)
			if err != nil {
				return err
			}
			recoveredStages = append(recoveredStages, saved)
		}
		return nil
	})
	return recoveredRuns, recoveredStages, err
}

func (r *SQLHarnessRepository) InterruptRunning(ctx context.Context, now time.Time) ([]domain.AgentRun, error) {
	var interrupted []domain.AgentRun
	err := r.Atomic(ctx, func(repository domain.Repository) error {
		tx := repository.(*SQLHarnessRepository)
		running, err := tx.ListAgentRuns(ctx, domain.AgentRunFilter{States: []domain.AgentRunState{domain.AgentRunning}, Limit: 1000})
		if err != nil {
			return err
		}
		for _, agentRun := range running {
			expect := domain.Expect{State: string(agentRun.State), Revision: agentRun.Revision}
			agentRun.State = domain.AgentInterrupted
			agentRun.LeaseOwner = ""
			agentRun.LeaseUntil = nil
			agentRun.FailureReason = "the server restarted while the agent ran"
			saved, err := tx.SaveAgentRun(ctx, agentRun, expect)
			if err != nil {
				return err
			}
			interrupted = append(interrupted, saved)
		}
		rows, err := tx.query(ctx, `SELECT `+stageColumns+` FROM harness_stages WHERE state = ? AND lease_owner <> ''`, string(domain.StageRunning))
		if err != nil {
			return err
		}
		var leased []domain.Stage
		for rows.Next() {
			stage, err := scanStage(rows)
			if err != nil {
				rows.Close()
				return err
			}
			leased = append(leased, stage)
		}
		rows.Close()
		for _, stage := range leased {
			expect := domain.Expect{State: string(stage.State), Revision: stage.Revision}
			stage.State = domain.StagePending
			stage.LeaseOwner = ""
			stage.LeaseUntil = nil
			if _, err := tx.SaveStage(ctx, stage, expect); err != nil {
				return err
			}
		}
		return nil
	})
	return interrupted, err
}

// --- approvals ---

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

// approvalRecord keeps the fields the API never shows.
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

// --- events ---

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

// --- outbox ---

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

// --- settings ---

const harnessSettingsID = "harness"

func (r *SQLHarnessRepository) LoadSettings(ctx context.Context) (domain.Settings, error) {
	var data string
	var revision int64
	err := r.queryRow(ctx, `SELECT settings_json, revision FROM harness_settings WHERE id = ?`, harnessSettingsID).Scan(&data, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DefaultSettings(), nil
	}
	if err != nil {
		return domain.Settings{}, err
	}
	settings := domain.DefaultSettings()
	if err := json.Unmarshal([]byte(data), &settings); err != nil {
		return domain.Settings{}, err
	}
	settings.Revision = revision
	normalized, err := settings.Normalize()
	if err != nil {
		return settings, nil
	}
	return normalized, nil
}

func (r *SQLHarnessRepository) SaveSettings(ctx context.Context, settings domain.Settings, expectedRevision int64) (domain.Settings, error) {
	now := time.Now().UTC()
	settings.Revision = expectedRevision + 1
	settings.UpdatedAt = &now
	data, err := json.Marshal(settings)
	if err != nil {
		return settings, err
	}
	if expectedRevision == 0 {
		_, err = r.exec(ctx, `INSERT INTO harness_settings (id, settings_json, revision, updated_at) VALUES (?, ?, ?, ?)`, harnessSettingsID, string(data), settings.Revision, harnessTime(now))
		if isUniqueViolation(err) {
			return settings, domain.ErrStateMoved
		}
		return settings, err
	}
	err = conditional(r.exec(ctx, `UPDATE harness_settings SET settings_json = ?, revision = ?, updated_at = ? WHERE id = ? AND revision = ?`, string(data), settings.Revision, harnessTime(now), harnessSettingsID, expectedRevision))
	return settings, err
}

// --- cost ---

func (r *SQLHarnessRepository) CostSummary(ctx context.Context, query domain.CostQuery) (domain.CostSummary, error) {
	var key string
	switch query.GroupBy {
	case domain.CostByDay:
		key = `SUBSTR(COALESCE(finished_at, queued_at), 1, 10)`
	case domain.CostByRun:
		key = `run_id`
	case domain.CostByRole:
		key = `role`
	case domain.CostByModel:
		key = `engine || ':' || model`
	default:
		return domain.CostSummary{}, domain.Invalid("groupBy must be day, run, role or model")
	}
	summary := domain.CostSummary{GroupBy: query.GroupBy, From: query.From, To: query.To, Rows: []domain.CostRow{}}
	rows, err := r.query(ctx, `SELECT `+key+` AS group_key, COALESCE(SUM(cost_usd), 0), COALESCE(SUM(turns), 0), COUNT(*) FROM harness_agent_runs WHERE COALESCE(finished_at, queued_at) >= ? AND COALESCE(finished_at, queued_at) < ? GROUP BY `+key+` ORDER BY group_key ASC`, harnessTime(query.From), harnessTime(query.To))
	if err != nil {
		return summary, err
	}
	defer rows.Close()
	for rows.Next() {
		var row domain.CostRow
		if err := rows.Scan(&row.Key, &row.CostUSD, &row.Turns, &row.AgentRuns); err != nil {
			return summary, err
		}
		row.Label = row.Key
		summary.TotalUSD += row.CostUSD
		summary.Rows = append(summary.Rows, row)
	}
	return summary, rows.Err()
}

var _ domain.Repository = (*SQLHarnessRepository)(nil)
