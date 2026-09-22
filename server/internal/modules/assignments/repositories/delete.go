package repositories

import (
	"context"
	"errors"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const entityAssignment = "assignment"

// Delete removes an inactive assignment while preserving assigned work and retained history.
func (s *Postgres) Delete(ctx context.Context, req domain.Request, now time.Time) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var allowed bool
	err = tx.QueryRow(ctx, `SELECT published_at IS NULL OR closed_at IS NOT NULL OR closes_at <= $2 FROM app.assignments WHERE id = $1 FOR UPDATE`, req.ID, now).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if !allowed {
		return domain.ErrNotArchived
	}

	if _, err := tx.Exec(ctx, `DELETE FROM app.assignments WHERE id = $1`, req.ID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
			return domain.ErrReferenced
		}
		return err
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID, Action: "assignment.deleted", Entity: entityAssignment, EntityID: &req.ID,
		OccurredAt: now, IP: opt.String(req.IP), UserAgent: opt.String(req.UserAgent),
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
