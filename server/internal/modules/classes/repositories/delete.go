package repositories

import (
	"context"
	"errors"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"time"

	"github.com/jackc/pgx/v5"
)

const entityClass = "class"

var classReferences = map[string]domain.Reference{
	"assignment_classes_class_id_fkey": domain.ReferencedByAssignments,
	"class_members_join_code_id_fkey":  domain.ReferencedByMembers,
}

// ClassReferencedBy names what a foreign key holds when it refuses a class's
// deletion, directly or through the cascade to its join codes. A constraint
// the map does not name is ReferencedByOther, so the refusal is still a 409.
func ClassReferencedBy(constraint string) domain.Reference {
	if by, ok := classReferences[constraint]; ok {
		return by
	}
	return domain.ReferencedByOther
}

// Delete removes an inactive class the actor teaches, or any with the actor's
// scope.all, while preserving assigned work and retained history. Another
// teacher's class answers ErrNotFound before either refusal can reveal it.
func (s *Postgres) Delete(ctx context.Context, classID string, by actor.Actor, now time.Time) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var allowed bool
	err = tx.QueryRow(ctx, `SELECT archived_at IS NOT NULL FROM app.classes WHERE id = $1 AND `+taughtClass+` FOR UPDATE`,
		classID, by.Scope.All, opt.String(by.ID)).Scan(&allowed)
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
		if constraint, ok := db.ForeignKeyViolation(err); ok {
			return &domain.ReferencedError{By: ClassReferencedBy(constraint)}
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
