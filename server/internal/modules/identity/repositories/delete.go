package repositories

import (
	"context"
	"errors"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/visibility"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Delete removes an inactive student the actor reaches, or any with All, while
// preserving assigned work and retained history. Another teacher's student
// answers ErrStudentNotFound before either refusal can reveal it.
func (s *Students) Delete(ctx context.Context, req domain.WriteRequest, id string, now time.Time) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var allowed bool
	err = tx.QueryRow(ctx, `SELECT u.disabled_at IS NOT NULL FROM app.users u
		 WHERE u.id = $1 AND u.role = 'student' AND ($2::boolean OR u.id IN `+visibility.StudentIDs(3)+`) FOR UPDATE`,
		id, req.All, opt.String(req.ActorID)).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrStudentNotFound
	}
	if err != nil {
		return err
	}
	if !allowed {
		return domain.ErrNotArchived
	}

	var retained bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM app.audit_log WHERE actor_user_id = $1)`, id).Scan(&retained); err != nil {
		return err
	}
	if retained {
		return domain.ErrReferenced
	}

	if _, err := tx.Exec(ctx, `DELETE FROM app.users WHERE id = $1`, id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
			return domain.ErrReferenced
		}
		return err
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID, Action: "student.deleted", Entity: "student", EntityID: &id,
		OccurredAt: now, IP: opt.String(req.IP), UserAgent: opt.String(req.UserAgent),
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
