package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/schedule"
	"quizzivy/internal/shared/visibility"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const overrideChangesCheck = "assignment_student_overrides_changes_check"

// Extend moves the close of an assignment the actor reaches later by minutes,
// for everyone, and records it in the statement that moves it. Only an
// assignment that has not closed qualifies, judged at the database's clock as
// Reopen judges it, so exactly one of the two takes any given assignment; a
// closed one answers ErrClosed and another teacher's answers ErrNotFound.
func (s *Postgres) Extend(ctx context.Context, req domain.Request, minutes int, notify bool, now time.Time) (domain.Assignment, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: begin extend: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var extended string
	err = tx.QueryRow(ctx, `
		WITH updated AS (
		  UPDATE app.assignments a
		     SET closes_at = a.closes_at + make_interval(mins => $2::int)
		   WHERE a.id = $1::uuid AND `+schedule.DerivedStatus+` <> 'closed'
		     AND ($3::boolean OR a.id IN `+visibility.AssignmentIDs(4)+`)
		  RETURNING a.id, old.closes_at AS prev_closes_at, new.closes_at AS next_closes_at
		), logged AS (
		  INSERT INTO app.audit_log
		         (actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff)
		  SELECT $4::uuid, 'assignment.extended', 'assignment', updated.id, $5, nullif($6, '')::inet, nullif($7, ''),
		         jsonb_build_object(
		           'closes_at', jsonb_build_object('old', updated.prev_closes_at, 'new', updated.next_closes_at),
		           'minutes', $2::int, 'notify', $8::boolean)
		    FROM updated
		)
		SELECT id::text FROM updated`,
		req.ID, minutes, req.All, opt.String(req.ActorID), now, req.IP, req.UserAgent, notify).Scan(&extended)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Assignment{}, whyNotExtended(ctx, tx, req.Scope(), req.ID)
	}
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: extend: %w", err)
	}

	saved, err := s.get(ctx, tx, req.Scope(), req.ID, false)
	if err != nil {
		return domain.Assignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: commit extend: %w", err)
	}
	return saved, nil
}

func assignmentReached(ctx context.Context, q db.Querier, scope access.Scope, id string) (bool, error) {
	var reached bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM app.assignments WHERE id = $1::uuid AND ($2::boolean OR id IN `+visibility.AssignmentIDs(3)+`))`,
		id, scope.All, opt.String(scope.UserID)).Scan(&reached)
	if err != nil {
		return false, fmt.Errorf("assignments: reach check: %w", err)
	}
	return reached, nil
}

func whyNotExtended(ctx context.Context, q db.Querier, scope access.Scope, id string) error {
	reached, err := assignmentReached(ctx, q, scope, id)
	if err != nil {
		return err
	}
	if !reached {
		return domain.ErrNotFound
	}
	return domain.ErrClosed
}

// SetOverrides gives each named student an override on an assignment the
// actor reaches, or changes the one they have, and records each in the
// statement that writes it. The students must be targets of the assignment
// that the actor reaches; if any is not, nothing is written and the error
// names them. The assignment row is held for the transaction, so concurrent
// writes to one assignment's window and overrides take turns.
func (s *Postgres) SetOverrides(ctx context.Context, req domain.Request, in domain.OverrideInput) ([]domain.StudentOverride, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	tx, err := s.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("assignments: begin set overrides: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockForOverrides(ctx, tx, req.Scope(), req.ID); err != nil {
		return nil, err
	}
	if err := checkOverrideTargets(ctx, tx, req, in.StudentIDs); err != nil {
		return nil, err
	}
	if in.ExtendBy != nil {
		if err := refuseClosedStudents(ctx, tx, req.ID, in.StudentIDs); err != nil {
			return nil, err
		}
	}
	written, err := writeOverrides(ctx, tx, req, in)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("assignments: commit set overrides: %w", err)
	}
	return written, nil
}

func lockForOverrides(ctx context.Context, tx pgx.Tx, scope access.Scope, id string) error {
	var locked int
	err := tx.QueryRow(ctx, `
		SELECT 1 FROM app.assignments
		 WHERE id = $1::uuid AND ($2::boolean OR id IN `+visibility.AssignmentIDs(3)+`)
		   FOR NO KEY UPDATE`,
		id, scope.All, opt.String(scope.UserID)).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("assignments: lock for overrides: %w", err)
	}
	return nil
}

func checkOverrideTargets(ctx context.Context, tx pgx.Tx, req domain.Request, studentIDs []string) error {
	missing, err := missingIDs(ctx, tx, `
		SELECT u.id::text FROM app.users u
		 WHERE u.id = ANY($1::uuid[])
		   AND u.disabled_at IS NULL
		   AND u.role_id IN (SELECT r.id FROM app.student_like_roles r)
		   AND u.id IN (
		       SELECT m.user_id
		         FROM app.assignment_classes ac
		         JOIN app.class_members m ON m.class_id = ac.class_id
		        WHERE ac.assignment_id = $2::uuid
		       UNION
		       SELECT ast.user_id FROM app.assignment_students ast
		        WHERE ast.assignment_id = $2::uuid)
		   AND ($3::boolean OR u.id IN `+visibility.StudentIDs(4)+`)`,
		studentIDs, req.ID, req.All, opt.String(req.ActorID))
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return &domain.NotTargetedError{StudentIDs: missing}
	}
	return nil
}

func refuseClosedStudents(ctx context.Context, tx pgx.Tx, assignmentID string, studentIDs []string) error {
	var closed bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		    FROM unnest($2::uuid[]) AS s(student_id)
		    JOIN app.assignments a ON a.id = $1::uuid
		    `+schedule.OverrideJoin("s.student_id")+`
		   WHERE `+schedule.CloseOf("o")+` <= now())`,
		assignmentID, studentIDs).Scan(&closed)
	if err != nil {
		return fmt.Errorf("assignments: check student closes: %w", err)
	}
	if closed {
		return domain.ErrClosed
	}
	return nil
}

func writeOverrides(ctx context.Context, tx pgx.Tx, req domain.Request, in domain.OverrideInput) ([]domain.StudentOverride, error) {
	rows, err := tx.Query(ctx, `
		WITH wanted AS (
		  SELECT s.student_id,
		         coalesce(CASE WHEN $3::int IS NULL THEN $4::timestamptz
		                       ELSE `+schedule.CloseOf("o")+` + make_interval(mins => $3::int) END,
		                  o.closes_at) AS closes_at,
		         coalesce($5::int, o.duration_minutes) AS duration_minutes,
		         coalesce($6::int, o.extra_attempts, 0) AS extra_attempts
		    FROM unnest($2::uuid[]) AS s(student_id)
		    JOIN app.assignments a ON a.id = $1::uuid
		    `+schedule.OverrideJoin("s.student_id")+`
		), upserted AS (
		  INSERT INTO app.assignment_student_overrides AS ov
		         (assignment_id, student_id, closes_at, duration_minutes, extra_attempts, reason, created_by)
		  SELECT $1::uuid, w.student_id, w.closes_at, w.duration_minutes, w.extra_attempts, $7::text, $8::uuid
		    FROM wanted w
		  ON CONFLICT (assignment_id, student_id) DO UPDATE
		     SET closes_at = EXCLUDED.closes_at,
		         duration_minutes = EXCLUDED.duration_minutes,
		         extra_attempts = EXCLUDED.extra_attempts,
		         reason = EXCLUDED.reason
		  RETURNING ov.student_id, ov.closes_at, ov.duration_minutes, ov.extra_attempts, ov.reason,
		            ov.created_at, ov.updated_at,
		            old.closes_at AS prev_closes_at, old.duration_minutes AS prev_duration,
		            old.extra_attempts AS prev_extra
		), logged AS (
		  INSERT INTO app.audit_log
		         (actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff)
		  SELECT $8::uuid, 'assignment.override_set', 'assignment', $1::uuid, $9, nullif($10, '')::inet, nullif($11, ''),
		         jsonb_build_object(
		           'studentId', u.student_id,
		           'closes_at', jsonb_build_object('old', u.prev_closes_at, 'new', u.closes_at),
		           'duration_minutes', jsonb_build_object('old', u.prev_duration, 'new', u.duration_minutes),
		           'extra_attempts', jsonb_build_object('old', u.prev_extra, 'new', u.extra_attempts),
		           'reason', u.reason, 'notify', $12::boolean)
		    FROM upserted u
		)
		SELECT u.student_id::text, s.full_name, u.closes_at, u.duration_minutes, u.extra_attempts,
		       u.reason, u.created_at, u.updated_at
		  FROM upserted u
		  JOIN app.users s ON s.id = u.student_id
		 ORDER BY s.full_name, u.student_id`,
		req.ID, in.StudentIDs, in.ExtendBy, in.ClosesAt, in.DurationMin, in.ExtraAttempts,
		in.CleanReason(), opt.String(req.ActorID), in.Now, req.IP, req.UserAgent, in.Notify)
	if err != nil {
		return nil, overrideWriteError(err)
	}
	written, err := pgx.CollectRows(rows, scanOverride)
	if err != nil {
		return nil, overrideWriteError(err)
	}
	return written, nil
}

func overrideWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == overrideChangesCheck {
		return &domain.ValidationError{Fields: []domain.FieldError{{
			Field:   "extraAttempts",
			Message: "Học viên không còn thay đổi nào. Hãy gỡ ngoại lệ thay vì đặt về 0.",
		}}}
	}
	return fmt.Errorf("assignments: write overrides: %w", err)
}

func scanOverride(row pgx.CollectableRow) (domain.StudentOverride, error) {
	var o domain.StudentOverride
	err := row.Scan(&o.StudentID, &o.StudentName, &o.ClosesAt, &o.DurationMin, &o.ExtraAttempts,
		&o.Reason, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

// Overrides lists the overrides on an assignment the scope reaches, for the
// students the scope reaches, by student name. Another teacher's assignment
// answers ErrNotFound as a missing one does.
func (s *Postgres) Overrides(ctx context.Context, scope access.Scope, assignmentID string) ([]domain.StudentOverride, error) {
	reached, err := assignmentReached(ctx, s.Conn(), scope, assignmentID)
	if err != nil {
		return nil, err
	}
	if !reached {
		return nil, domain.ErrNotFound
	}

	rows, err := s.Query(ctx, `
		SELECT ov.student_id::text, s.full_name, ov.closes_at, ov.duration_minutes, ov.extra_attempts,
		       ov.reason, ov.created_at, ov.updated_at
		  FROM app.assignment_student_overrides ov
		  JOIN app.users s ON s.id = ov.student_id
		 WHERE ov.assignment_id = $1::uuid
		   AND ($2::boolean OR ov.student_id IN `+visibility.StudentIDs(3)+`)
		 ORDER BY s.full_name, ov.student_id`,
		assignmentID, scope.All, opt.String(scope.UserID))
	if err != nil {
		return nil, fmt.Errorf("assignments: list overrides: %w", err)
	}
	found, err := pgx.CollectRows(rows, scanOverride)
	if err != nil {
		return nil, fmt.Errorf("assignments: list overrides: %w", err)
	}
	if found == nil {
		found = []domain.StudentOverride{}
	}
	return found, nil
}

// DeleteOverride takes one student's override off an assignment the actor
// reaches and records the values it removed, in one statement. An override the
// actor does not reach, or that does not exist, answers ErrNotFound.
func (s *Postgres) DeleteOverride(ctx context.Context, req domain.Request, studentID string, now time.Time) error {
	var removed int
	err := s.QueryRow(ctx, `
		WITH removed AS (
		  DELETE FROM app.assignment_student_overrides ov
		   WHERE ov.assignment_id = $1::uuid AND ov.student_id = $2::uuid
		     AND ($3::boolean OR ov.assignment_id IN `+visibility.AssignmentIDs(4)+`)
		     AND ($3::boolean OR ov.student_id IN `+visibility.StudentIDs(4)+`)
		  RETURNING ov.student_id, ov.closes_at, ov.duration_minutes, ov.extra_attempts, ov.reason
		), logged AS (
		  INSERT INTO app.audit_log
		         (actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff)
		  SELECT $4::uuid, 'assignment.override_removed', 'assignment', $1::uuid, $5, nullif($6, '')::inet, nullif($7, ''),
		         jsonb_build_object(
		           'studentId', r.student_id,
		           'closes_at', jsonb_build_object('old', r.closes_at, 'new', NULL),
		           'duration_minutes', jsonb_build_object('old', r.duration_minutes, 'new', NULL),
		           'extra_attempts', jsonb_build_object('old', r.extra_attempts, 'new', 0),
		           'reason', r.reason)
		    FROM removed r
		)
		SELECT count(*) FROM removed`,
		req.ID, studentID, req.All, opt.String(req.ActorID), now, req.IP, req.UserAgent).Scan(&removed)
	if err != nil {
		return fmt.Errorf("assignments: delete override: %w", err)
	}
	if removed == 0 {
		return domain.ErrNotFound
	}
	return nil
}
