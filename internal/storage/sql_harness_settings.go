package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"kode-stream/internal/harness/domain"
)

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
