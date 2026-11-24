package storage

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"kode-stream/internal/harness/domain"
)

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

var _ domain.Repository = (*SQLHarnessRepository)(nil)
