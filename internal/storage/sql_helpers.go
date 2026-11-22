package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type sqlRow interface {
	Scan(...any) error
}

func execSQL(db *sql.DB, driverName, query string, args ...any) (sql.Result, error) {
	return db.Exec(rebindSQL(driverName, query), args...)
}

func querySQL(db *sql.DB, driverName, query string, args ...any) (*sql.Rows, error) {
	return db.Query(rebindSQL(driverName, query), args...)
}

func queryRowSQL(db *sql.DB, driverName, query string, args ...any) sqlRow {
	return db.QueryRow(rebindSQL(driverName, query), args...)
}

func execTx(tx *sql.Tx, driverName, query string, args ...any) (sql.Result, error) {
	return tx.Exec(rebindSQL(driverName, query), args...)
}

func rebindSQL(driverName, query string) string {
	if driverName != StorageDriverPostgres {
		return query
	}
	var builder strings.Builder
	ordinal := 1
	for _, char := range query {
		if char == '?' {
			builder.WriteString(fmt.Sprintf("$%d", ordinal))
			ordinal++
			continue
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func encodeJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	return string(data), err
}

func formatTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func databaseBoolValue(driver string, value bool) any {
	if driver == StorageDriverPostgres {
		return value
	}
	return boolInt(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
