package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Conn is what a repository runs its SQL on: the pool in production, a
// transaction in tests that need one isolated snapshot.
type Conn interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Querier is the surface a pool and a transaction share, so one query helper
// serves both.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Context is the database context every module's repository is built on: one
// connection source and the transaction discipline around it.
type Context struct {
	conn Conn
}

func NewContext(conn Conn) Context { return Context{conn: conn} }

func (c Context) Conn() Conn { return c.conn }

func (c Context) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return c.conn.Exec(ctx, sql, args...)
}

func (c Context) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return c.conn.Query(ctx, sql, args...)
}

func (c Context) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return c.conn.QueryRow(ctx, sql, args...)
}

func (c Context) Begin(ctx context.Context) (pgx.Tx, error) { return c.conn.Begin(ctx) }

// InTx runs fn inside one transaction: committed when fn returns nil, rolled
// back otherwise, and the error names the unit of work.
func (c Context) InTx(ctx context.Context, name string, fn func(tx pgx.Tx) error) error {
	tx, err := c.conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin: %w", name, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit: %w", name, err)
	}
	return nil
}

// ErrNoRow is what QueryOne answers when the statement matched nothing; a
// repository maps it to its domain's not-found error.
var ErrNoRow = errors.New("db: no row")

// QueryOne runs a statement expected to yield one row and scans it with scan.
func QueryOne[T any](ctx context.Context, q Querier, sql string, args []any, scan func(pgx.Row) (T, error)) (T, error) {
	out, err := scan(q.QueryRow(ctx, sql, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		var zero T
		return zero, ErrNoRow
	}
	return out, err
}

// QueryMany runs a statement and scans every row with scan, in order.
func QueryMany[T any](ctx context.Context, q Querier, sql string, args []any, scan func(pgx.Rows) (T, error)) ([]T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func Count(ctx context.Context, q Querier, sql string, args ...any) (int, error) {
	var n int
	err := q.QueryRow(ctx, sql, args...).Scan(&n)
	return n, err
}

func Exists(ctx context.Context, q Querier, sql string, args ...any) (bool, error) {
	var found bool
	err := q.QueryRow(ctx, `SELECT EXISTS (`+sql+`)`, args...).Scan(&found)
	return found, err
}

// IsUniqueViolation reports whether err is Postgres refusing a duplicate on
// the named constraint (any constraint when name is empty).
func IsUniqueViolation(err error, name string) bool {
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != pgerrcode.UniqueViolation {
		return false
	}
	return name == "" || pg.ConstraintName == name
}

// EscapeLike makes user text safe inside a LIKE pattern with ESCAPE '\'.
func EscapeLike(s string) string { return likeEscaper.Replace(s) }
