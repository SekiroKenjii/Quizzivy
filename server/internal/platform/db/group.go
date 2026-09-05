package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Querier is the one method the helpers need, which a pool and a transaction
// both have.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// GroupBy runs one query and appends each scanned value under the key scan
// returns: the "children of a whole page in one round trip" shape §13.8 asks
// of every store, written once.
func GroupBy[T any](ctx context.Context, q Querier, sql string, args []any,
	scan func(pgx.Rows) (string, T, error)) (map[string][]T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]T{}
	for rows.Next() {
		key, value, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out[key] = append(out[key], value)
	}
	return out, rows.Err()
}
