package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/visibility"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
)

const entityUser = "user"

// Create adds a student who signs in with a temporary password (§6.3: only
// Google self-signup exists, so an admin-created account has to carry one),
// created by the actor. Every class it names must be one the actor teaches, or
// any with All; otherwise nothing is written and it answers ErrClassNotFound,
// before the email is checked.
func (s *Students) Create(ctx context.Context, req domain.WriteRequest, in domain.NewStudent) (domain.Student, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Student{}, fmt.Errorf("students: begin create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockTaughtClasses(ctx, tx, req, in.ClassIDs); err != nil {
		return domain.Student{}, err
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO app.users (email, full_name, role_id, password_hash, must_change_password, created_by)
		VALUES ($1, $2, (SELECT r.id FROM app.roles r WHERE r.builtin_key = 'student'), $3, $5, $4::uuid)
		RETURNING id::text`, in.Email, in.FullName, in.Hash, req.ActorID, !req.KeepTemporaryPassword).Scan(&id)
	if db.IsUniqueViolation(err, "") {
		return domain.Student{}, domain.ErrEmailTaken
	}
	if err != nil {
		return domain.Student{}, fmt.Errorf("students: insert: %w", err)
	}

	for _, classID := range in.ClassIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
			VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)
			ON CONFLICT (class_id, user_id) DO NOTHING`,
			classID, id, req.ActorID); err != nil {
			return domain.Student{}, fmt.Errorf("students: enrol: %w", err)
		}
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "student.created",
		Entity:      entityUser,
		EntityID:    &id,
		OccurredAt:  in.Now,
		IP:          opt.String(req.IP),
		UserAgent:   opt.String(req.UserAgent),
	}); err != nil {
		return domain.Student{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Student{}, fmt.Errorf("students: commit create: %w", err)
	}
	return s.get(ctx, req.Scope(), id, true)
}

func lockTaughtClasses(ctx context.Context, tx pgx.Tx, req domain.WriteRequest, classIDs []string) error {
	wanted := slices.Compact(slices.Sorted(slices.Values(classIDs)))
	if len(wanted) == 0 {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT c.id::text FROM app.classes c
		 WHERE c.id = ANY($1::uuid[]) AND ($2::boolean OR c.teacher_id = $3::uuid)
		 ORDER BY c.id FOR SHARE`, wanted, req.All, opt.String(req.ActorID))
	if err != nil {
		return fmt.Errorf("students: lock classes: %w", err)
	}
	taught, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("students: lock classes: %w", err)
	}
	if len(taught) != len(wanted) {
		return domain.ErrClassNotFound
	}
	return nil
}

// Update edits profile fields of a student the actor reaches, or disables the
// account; another teacher's student answers ErrStudentNotFound and is never
// touched. A disable revokes every refresh family and every unused set-password
// link the student has and moves the session epoch, so no session and no link,
// live or idle, survives it, including after the account is enabled again.
func (s *Students) Update(ctx context.Context, req domain.WriteRequest, in domain.StudentPatch) (domain.Student, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Student{}, fmt.Errorf("students: begin update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if in.Email != nil && !req.ManagesUsers() {
		if err := unshared(ctx, tx, req, in.ID, in.Email, false); err != nil {
			return domain.Student{}, err
		}
	}

	tag, err := tx.Exec(ctx, `
		UPDATE app.users u
		   SET full_name   = coalesce($2, full_name),
		       email       = coalesce($3, email),
		       disabled_at = CASE
		                       WHEN $4::boolean IS NULL THEN disabled_at
		                       WHEN $4 THEN coalesce(disabled_at, $5)
		                       ELSE NULL
		                     END,
		       session_epoch = session_epoch + CASE WHEN $4::boolean IS TRUE THEN 1 ELSE 0 END
		 WHERE u.id = $1::uuid AND `+studentLike+`
		   AND ($6::boolean OR u.id IN `+visibility.StudentIDs(7)+`)`,
		in.ID, in.FullName, in.Email, in.Disabled, in.Now, req.All, opt.String(req.ActorID))
	if db.IsUniqueViolation(err, "") {
		return domain.Student{}, domain.ErrEmailTaken
	}
	if errors.Is(UserWriteError(err), domain.ErrLastAdmin) {
		return domain.Student{}, domain.ErrLastAdmin
	}
	if err != nil {
		return domain.Student{}, fmt.Errorf("students: update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Student{}, domain.ErrStudentNotFound
	}
	if in.Disabled != nil && *in.Disabled {
		if _, err := tx.Exec(ctx, `
			UPDATE app.refresh_tokens
			   SET revoked_at = $2
			 WHERE user_id = $1::uuid AND revoked_at IS NULL`, in.ID, in.Now); err != nil {
			return domain.Student{}, fmt.Errorf("students: revoke sessions: %w", err)
		}
		if _, err := tx.Exec(ctx, revokeUnusedLinks, in.ID, in.Now); err != nil {
			return domain.Student{}, fmt.Errorf("students: revoke set-password links: %w", err)
		}
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "student.updated",
		Entity:      entityUser,
		EntityID:    &in.ID,
		OccurredAt:  in.Now,
		IP:          opt.String(req.IP),
		UserAgent:   opt.String(req.UserAgent),
	}); err != nil {
		return domain.Student{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Student{}, fmt.Errorf("students: commit update: %w", err)
	}

	return s.get(ctx, req.Scope(), in.ID, true)
}

// ResetPassword sets a temporary password on a student the actor reaches,
// revokes every session the student has and moves the session epoch, so a live
// access token stops working too. Another teacher's student answers
// ErrStudentNotFound; a student someone else also reaches answers
// ErrStudentShared unless the actor manages accounts.
func (s *Students) ResetPassword(ctx context.Context, req domain.WriteRequest, id, hash string, now time.Time) error {
	diff, err := batchDiff(req.BatchID)
	if err != nil {
		return err
	}
	tx, err := s.Begin(ctx)
	if err != nil {
		return fmt.Errorf("students: begin reset: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if !req.ManagesUsers() {
		if err := unshared(ctx, tx, req, id, nil, true); err != nil {
			return err
		}
	}
	tag, err := tx.Exec(ctx, `
		UPDATE app.users u
		   SET password_hash = $2, must_change_password = $5::boolean, session_epoch = session_epoch + 1
		 WHERE u.id = $1::uuid AND `+studentLike+` AND u.disabled_at IS NULL
		   AND ($3::boolean OR u.id IN `+visibility.StudentIDs(4)+`)`, id, hash, req.All, opt.String(req.ActorID), !req.KeepTemporaryPassword)
	if err != nil {
		return fmt.Errorf("students: reset password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrStudentNotFound
	}

	if _, err := tx.Exec(ctx, `
		UPDATE app.refresh_tokens
		   SET revoked_at = $2
		 WHERE user_id = $1::uuid AND revoked_at IS NULL`, id, now); err != nil {
		return fmt.Errorf("students: revoke sessions: %w", err)
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "student.password_reset",
		Entity:      entityUser,
		EntityID:    &id,
		OccurredAt:  now,
		IP:          opt.String(req.IP),
		UserAgent:   opt.String(req.UserAgent),
		Diff:        diff,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func batchDiff(batchID string) ([]byte, error) {
	if batchID == "" {
		return nil, nil
	}
	diff, err := json.Marshal(struct {
		BatchID string `json:"batchId"`
	}{batchID})
	if err != nil {
		return nil, fmt.Errorf("students: encode batch id: %w", err)
	}
	return diff, nil
}

// CheckResettable answers what ResetPassword would answer for the student on
// the grounds that need no write: ErrStudentNotFound for one the actor does not
// reach or who is disabled, and ErrStudentShared for one another teacher also
// reaches unless the actor manages accounts. It takes no lock, so only
// ResetPassword's own answer is final; a caller uses this one to decline a
// student before it spends a password hash on it.
func (s *Students) CheckResettable(ctx context.Context, req domain.WriteRequest, id string) error {
	err := checkUnshared(ctx, s, req, id, nil, true, ``)
	if req.ManagesUsers() && errors.Is(err, domain.ErrStudentShared) {
		return nil
	}
	return err
}

func unshared(ctx context.Context, tx pgx.Tx, req domain.WriteRequest, id string, email *string, activeOnly bool) error {
	return checkUnshared(ctx, tx, req, id, email, activeOnly, ` FOR NO KEY UPDATE OF u`)
}

func checkUnshared(ctx context.Context, q db.Querier, req domain.WriteRequest, id string, email *string, activeOnly bool, lock string) error {
	active := ``
	if activeOnly {
		active = ` AND u.disabled_at IS NULL`
	}
	var alone bool
	err := q.QueryRow(ctx, `
		SELECT u.email IS NOT DISTINCT FROM $4::text OR coalesce(
		         NOT EXISTS (SELECT 1 FROM app.class_members m WHERE m.user_id = u.id AND m.class_id NOT IN `+visibility.TaughtClassIDs(3)+`)
		         AND (u.created_by IS NULL OR u.created_by = $3::uuid)
		         AND NOT EXISTS (SELECT 1 FROM app.assignment_students s JOIN app.assignments x ON x.id = s.assignment_id
		                          WHERE s.user_id = u.id AND x.created_by IS DISTINCT FROM $3::uuid)
		         AND (EXISTS (SELECT 1 FROM app.class_members m WHERE m.user_id = u.id) OR u.created_by = $3::uuid),
		         false)
		  FROM app.users u
		 WHERE u.id = $1::uuid AND `+studentLike+active+`
		   AND ($2::boolean OR u.id IN `+visibility.StudentIDs(3)+`)`+lock, id, req.All, opt.String(req.ActorID), email).Scan(&alone)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrStudentNotFound
	}
	if err != nil {
		return fmt.Errorf("students: check sharing: %w", err)
	}
	if !alone {
		return domain.ErrStudentShared
	}
	return nil
}
