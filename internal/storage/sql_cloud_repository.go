package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/provider"
)

// SQLCloudRepository persists only durable Cloud metadata. Runtime connections
// remain owned by the HTTP/WebSocket process.
type SQLCloudRepository struct {
	db     *sql.DB
	driver string
	now    func() time.Time
}

func (r *SQLCloudRepository) ListWorkspaces(ctx context.Context, owner string) ([]models.WorkspaceConfig, error) {
	rows, err := r.db.QueryContext(ctx, rebindSQL(r.driver, `SELECT workspace_json FROM cloud_workspaces WHERE owner_user_id = ? ORDER BY updated_at DESC`), owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []models.WorkspaceConfig{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var workspace models.WorkspaceConfig
		if err := json.Unmarshal([]byte(raw), &workspace); err != nil {
			return nil, err
		}
		workspace.OwnerUserID = owner
		result = append(result, workspace)
	}
	return result, rows.Err()
}

func (r *SQLCloudRepository) GetWorkspace(ctx context.Context, owner, id string) (models.WorkspaceConfig, bool, error) {
	var raw string
	err := r.db.QueryRowContext(ctx, rebindSQL(r.driver, `SELECT workspace_json FROM cloud_workspaces WHERE owner_user_id = ? AND id = ?`), owner, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return models.WorkspaceConfig{}, false, nil
	}
	if err != nil {
		return models.WorkspaceConfig{}, false, err
	}
	var workspace models.WorkspaceConfig
	if err := json.Unmarshal([]byte(raw), &workspace); err != nil {
		return models.WorkspaceConfig{}, false, err
	}
	workspace.OwnerUserID = owner
	return workspace, true, nil
}

func (r *SQLCloudRepository) UpsertWorkspace(ctx context.Context, owner string, workspace models.WorkspaceConfig) (models.WorkspaceConfig, error) {
	workspace.OwnerUserID = owner
	raw, err := json.Marshal(workspace)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	_, err = r.db.ExecContext(ctx, rebindSQL(r.driver, `INSERT INTO cloud_workspaces (owner_user_id, id, workspace_json, updated_at) VALUES (?, ?, ?, ?) ON CONFLICT(owner_user_id, id) DO UPDATE SET workspace_json = excluded.workspace_json, updated_at = excluded.updated_at`), owner, workspace.ID, string(raw), formatTime(r.now().UTC()))
	return workspace, err
}

func (r *SQLCloudRepository) ListAgents(ctx context.Context, owner string) ([]models.CloudAgent, error) {
	rows, err := r.db.QueryContext(ctx, rebindSQL(r.driver, `SELECT agent_json FROM cloud_agents WHERE owner_user_id = ? ORDER BY last_seen_at DESC`), owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []models.CloudAgent{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var agent models.CloudAgent
		if err := json.Unmarshal([]byte(raw), &agent); err != nil {
			return nil, err
		}
		agent.UserID = owner
		result = append(result, agent)
	}
	return result, rows.Err()
}

func (r *SQLCloudRepository) UpsertAgent(ctx context.Context, owner string, agent models.CloudAgent) (models.CloudAgent, error) {
	agent.UserID = owner
	raw, err := json.Marshal(agent)
	if err != nil {
		return models.CloudAgent{}, err
	}
	_, err = r.db.ExecContext(ctx, rebindSQL(r.driver, `INSERT INTO cloud_agents (owner_user_id, id, agent_json, last_seen_at) VALUES (?, ?, ?, ?) ON CONFLICT(owner_user_id, id) DO UPDATE SET agent_json = excluded.agent_json, last_seen_at = excluded.last_seen_at`), owner, agent.ID, string(raw), formatTime(agent.LastSeenAt))
	return agent, err
}

func (r *SQLCloudRepository) GetProviderInstance(ctx context.Context, owner, id string) (provider.Instance, bool, error) {
	var raw string
	err := r.db.QueryRowContext(ctx, rebindSQL(r.driver, `SELECT instance_json FROM cloud_provider_instances WHERE owner_user_id = ? AND id = ?`), owner, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return provider.Instance{}, false, nil
	}
	if err != nil {
		return provider.Instance{}, false, err
	}
	var instance provider.Instance
	if err := json.Unmarshal([]byte(raw), &instance); err != nil {
		return provider.Instance{}, false, err
	}
	return instance, true, nil
}

func (r *SQLCloudRepository) UpsertProviderInstance(ctx context.Context, owner string, instance provider.Instance) error {
	raw, err := json.Marshal(instance)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, rebindSQL(r.driver, `INSERT INTO cloud_provider_instances (owner_user_id, id, instance_json, updated_at) VALUES (?, ?, ?, ?) ON CONFLICT(owner_user_id, id) DO UPDATE SET instance_json = excluded.instance_json, updated_at = excluded.updated_at`), owner, instance.ID, string(raw), formatTime(r.now().UTC()))
	return err
}

func (r *SQLCloudRepository) GetEncryptedConnection(ctx context.Context, owner, instanceID string) (string, bool, error) {
	var encrypted string
	err := r.db.QueryRowContext(ctx, rebindSQL(r.driver, `SELECT encrypted_credentials FROM cloud_provider_connections WHERE owner_user_id = ? AND provider_instance_id = ?`), owner, instanceID).Scan(&encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return encrypted, err == nil, err
}

func (r *SQLCloudRepository) SaveEncryptedConnection(ctx context.Context, owner, instanceID, encrypted string) error {
	_, err := r.db.ExecContext(ctx, rebindSQL(r.driver, `INSERT INTO cloud_provider_connections (owner_user_id, provider_instance_id, encrypted_credentials, connection_json, updated_at) VALUES (?, ?, ?, '{}', ?) ON CONFLICT(owner_user_id, provider_instance_id) DO UPDATE SET encrypted_credentials = excluded.encrypted_credentials, updated_at = excluded.updated_at`), owner, instanceID, encrypted, formatTime(r.now().UTC()))
	return err
}

func (r *SQLCloudRepository) RevokeConnection(ctx context.Context, owner, instanceID string) error {
	_, err := r.db.ExecContext(ctx, rebindSQL(r.driver, `DELETE FROM cloud_provider_connections WHERE owner_user_id = ? AND provider_instance_id = ?`), owner, instanceID)
	return err
}
