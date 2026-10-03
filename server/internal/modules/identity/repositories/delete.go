package repositories

import (
	"context"
	"errors"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/visibility"
	"time"

	"github.com/jackc/pgx/v5"
)

var userReferences = map[string]domain.Reference{
	"assignment_students_user_id_fkey":        domain.ReferencedByAssignments,
	"assignments_created_by_fkey":             domain.ReferencedByAssignments,
	"attempts_student_id_fkey":                domain.ReferencedByAttempts,
	"class_join_codes_created_by_fkey":        domain.ReferencedByOwnedContent,
	"classes_teacher_id_fkey":                 domain.ReferencedByOwnedContent,
	"media_assets_owner_id_fkey":              domain.ReferencedByOwnedContent,
	"media_assets_uploaded_by_fkey":           domain.ReferencedByOwnedContent,
	"question_groups_created_by_fkey":         domain.ReferencedByOwnedContent,
	"question_groups_owner_id_fkey":           domain.ReferencedByOwnedContent,
	"questions_created_by_fkey":               domain.ReferencedByOwnedContent,
	"questions_owner_id_fkey":                 domain.ReferencedByOwnedContent,
	"test_versions_published_by_fkey":         domain.ReferencedByOwnedContent,
	"tests_created_by_fkey":                   domain.ReferencedByOwnedContent,
	"tests_owner_id_fkey":                     domain.ReferencedByOwnedContent,
	"word_import_commits_committed_by_fkey":   domain.ReferencedByOwnedContent,
	"word_import_drafts_edited_by_fkey":       domain.ReferencedByOwnedContent,
	"word_import_run_events_actor_id_fkey":    domain.ReferencedByOwnedContent,
	"word_import_runs_requested_by_fkey":      domain.ReferencedByOwnedContent,
	"word_import_source_sets_created_by_fkey": domain.ReferencedByOwnedContent,
	"word_import_sources_uploaded_by_fkey":    domain.ReferencedByOwnedContent,
	"word_imports_created_by_fkey":            domain.ReferencedByOwnedContent,
}

// UserReferencedBy names what a foreign key to app.users holds when it refuses
// a user's deletion. A constraint the map does not name, such as one a later
// migration adds, is ReferencedByOther, so the refusal is still a 409.
func UserReferencedBy(constraint string) domain.Reference {
	if by, ok := userReferences[constraint]; ok {
		return by
	}
	return domain.ReferencedByOther
}

// UserWriteError maps what Postgres refuses on a write to app.users: the last
// active Admin (users_last_admin) is ErrLastAdmin, and a foreign key still
// holding the row is a ReferencedError naming what holds it. Any other error
// is returned unchanged.
func UserWriteError(err error) error {
	if db.IsCheckViolation(err, "users_last_admin") {
		return domain.ErrLastAdmin
	}
	if constraint, ok := db.ForeignKeyViolation(err); ok {
		return &domain.ReferencedError{By: UserReferencedBy(constraint)}
	}
	return err
}

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
		 WHERE u.id = $1 AND `+studentLike+` AND ($2::boolean OR u.id IN `+visibility.StudentIDs(3)+`) FOR UPDATE OF u`,
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
		return &domain.ReferencedError{By: domain.ReferencedByAudit}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM app.users WHERE id = $1`, id); err != nil {
		return UserWriteError(err)
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID, Action: "student.deleted", Entity: "student", EntityID: &id,
		OccurredAt: now, IP: opt.String(req.IP), UserAgent: opt.String(req.UserAgent),
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
