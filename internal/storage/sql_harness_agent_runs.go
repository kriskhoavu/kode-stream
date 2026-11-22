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
