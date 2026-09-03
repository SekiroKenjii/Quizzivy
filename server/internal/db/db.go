package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps pgxpool so callers depend on an interface we control.
type Pool struct {
	*pgxpool.Pool
}

// Open connects using the supplied DSN.
//
// The DSN must name quizzivy_app, never the owner (§13.5). The app role has DML
// on schema app and nothing else -- it cannot run DDL, and migration 00022
// revokes UPDATE and DELETE on the append-only tables, so "attempt_events is
// append-only" is a privilege rather than a promise.
func Open(ctx context.Context, dsn string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database dsn: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return &Pool{Pool: pool}, nil
}

// Ping reports whether the database is reachable. Backs /healthz, where a
// short timeout is right: a health check should fail fast, not hang.
func (p *Pool) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return p.Pool.Ping(ctx)
}

// WaitReady blocks until the database answers, or the budget runs out.
func (p *Pool) WaitReady(ctx context.Context, budget time.Duration) error {
	deadline := time.Now().Add(budget)
	var lastErr error
	for attempt := 1; ; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := p.Pool.Ping(attemptCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		if time.Now().After(deadline) {
			return fmt.Errorf("database not ready after %s (%d attempts): %w", budget, attempt, lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
