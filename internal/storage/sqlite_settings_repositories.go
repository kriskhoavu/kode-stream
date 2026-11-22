package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"kode-stream/internal/ai"
)

type SQLiteAISettingsRepository struct {
	db     *sql.DB
	driver string
	mu     sync.Mutex
}

type SQLiteImportStatusRepository struct {
	db     *sql.DB
	driver string
}

func (r *SQLiteAISettingsRepository) Load() (ai.Settings, error) {
	var raw string
	err := queryRowSQL(r.db, r.driver, `SELECT settings_json FROM ai_settings WHERE id = 'default'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return ai.Settings{}, nil
	}
	if err != nil {
		return ai.Settings{}, err
	}
	var settings ai.Settings
	return settings, json.Unmarshal([]byte(raw), &settings)
}

func (r *SQLiteAISettingsRepository) Save(settings ai.Settings) (ai.Settings, error) {
	raw, err := encodeJSON(settings)
	if err != nil {
		return ai.Settings{}, err
	}
	_, err = execSQL(r.db, r.driver, `INSERT INTO ai_settings (id, settings_json, updated_at) VALUES ('default', ?, ?) ON CONFLICT(id) DO UPDATE SET settings_json = excluded.settings_json, updated_at = excluded.updated_at`, raw, formatTime(time.Now().UTC()))
	return settings, err
}

func (r *SQLiteImportStatusRepository) ImportCompleted(source string) (bool, error) {
	var value string
	err := queryRowSQL(r.db, r.driver, `SELECT source_name FROM import_status WHERE source_name = ?`, source).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *SQLiteImportStatusRepository) MarkImportCompleted(source string, completedAt time.Time) error {
	_, err := execSQL(r.db, r.driver, `INSERT INTO import_status (source_name, completed_at) VALUES (?, ?) ON CONFLICT(source_name) DO UPDATE SET completed_at = excluded.completed_at`, source, formatTime(completedAt))
	return err
}
