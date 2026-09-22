package repositories

import (
	"context"
	"errors"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const entityClass = "class"

// Delete removes an inactive class while preserving assigned work and retained history.
func (s *Postgres) Delete(ctx context.Context, classID string, by actor.Actor, now time.Time) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var allowed bool
	err = tx.QueryRow(ctx, `SELECT archived_at IS NOT NULL FROM app.classes WHERE id = $1 FOR UPDATE`, classID).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if !allowed {
		return domain.ErrNotArchived
	}

	if _, err := tx.Exec(ctx, `DELETE FROM app.classes WHERE id = $1`, classID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
			return domain.ErrReferenced
		}
		return err
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &by.ID, Action: "class.deleted", Entity: entityClass, EntityID: &classID,
		OccurredAt: now, IP: opt.String(by.IP), UserAgent: opt.String(by.UserAgent),
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
