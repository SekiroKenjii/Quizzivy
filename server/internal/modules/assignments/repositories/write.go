package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Postgres) Create(ctx context.Context, req domain.Request, in domain.WriteInput) (domain.Assignment, error) {
	if err := in.Validate(); err != nil {
		return domain.Assignment{}, err
	}

	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: begin create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	testID, err := publishedTestFor(ctx, tx, in.TestVersionID)
	if err != nil {
		return domain.Assignment{}, err
	}
	if err := checkTargets(ctx, tx, in); err != nil {
		return domain.Assignment{}, err
	}

	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO app.assignments
		       (test_id, test_version_id, opens_at, closes_at, closed_at,
		        duration_minutes, max_attempts, shuffle_questions, shuffle_options,
		        review_show_score, review_show_correct_answers, review_show_explanations,
		        integrity_require_fullscreen, integrity_block_copy_paste,
		        integrity_max_focus_loss, integrity_on_limit_exceeded, integrity_min_away_ms,
		        created_by, published_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
		        $13, $14, $15, $16::app.integrity_action, $17, $18::uuid, $19)
		RETURNING id::text`,
		testID, in.TestVersionID, in.OpensAt, in.ClosesAt, domain.Schedule.ClosedAtOf(in),
		in.DurationMin, in.MaxAttempts, in.ShuffleQ, in.ShuffleO,
		in.Review.ShowScore, in.Review.ShowCorrectAnswers, in.Review.ShowExplanations,
		in.Integrity.RequireFullscreen, in.Integrity.BlockCopyPaste,
		in.Integrity.MaxFocusLoss, in.Integrity.OnLimitExceeded, in.Integrity.MinAwayMs,
		req.ActorID, domain.Schedule.PublishedAtOf(in)).Scan(&id); err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: insert: %w", err)
	}

	if err := writeTargets(ctx, tx, id, in); err != nil {
		return domain.Assignment{}, err
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "assignment.created",
		Entity:      "assignment",
		EntityID:    &id,
		OccurredAt:  in.Now,
		IP:          opt.String(req.IP),
		UserAgent:   opt.String(req.UserAgent),
	}); err != nil {
		return domain.Assignment{}, err
	}

	created, err := s.get(ctx, tx, id)
	if err != nil {
		return domain.Assignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: commit create: %w", err)
	}
	return created, nil
}

// versionStillFree refuses a version change once anyone has started.
func versionStillFree(ctx context.Context, tx pgx.Tx, assignmentID, next, current string) error {
	if next == current {
		return nil
	}
	var started bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM app.attempts WHERE assignment_id = $1::uuid)`,
		assignmentID).Scan(&started); err != nil {
		return fmt.Errorf("assignments: attempt check: %w", err)
	}
	if started {
		return domain.ErrVersionLocked
	}
	return nil
}

func (s *Postgres) Update(ctx context.Context, req domain.Request, in domain.WriteInput) (domain.Assignment, error) {
	if err := in.Validate(); err != nil {
		return domain.Assignment{}, err
	}

	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: begin update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := lockForUpdate(ctx, tx, req.ID)
	if err != nil {
		return domain.Assignment{}, err
	}
	testID, err := publishedTestFor(ctx, tx, in.TestVersionID)
	if err != nil {
		return domain.Assignment{}, err
	}
	if err := versionStillFree(ctx, tx, req.ID, in.TestVersionID, current.versionID); err != nil {
		return domain.Assignment{}, err
	}
	if err := checkTargets(ctx, tx, in); err != nil {
		return domain.Assignment{}, err
	}

	next := current.closedAt
	if in.CloseNow && next == nil {
		next = &in.Now
	}

	if _, err := tx.Exec(ctx, `
		UPDATE app.assignments
		   SET test_id = $2::uuid, test_version_id = $3::uuid,
		       opens_at = $4, closes_at = $5, closed_at = $6,
		       duration_minutes = $7, max_attempts = $8,
		       shuffle_questions = $9, shuffle_options = $10,
		       review_show_score = $11, review_show_correct_answers = $12,
		       review_show_explanations = $13,
		       integrity_require_fullscreen = $14, integrity_block_copy_paste = $15,
		       integrity_max_focus_loss = $16,
		       integrity_on_limit_exceeded = $17::app.integrity_action,
		       integrity_min_away_ms = $18,
		       published_at = $19
		 WHERE id = $1::uuid`,
		req.ID, testID, in.TestVersionID, in.OpensAt, in.ClosesAt, next,
		in.DurationMin, in.MaxAttempts, in.ShuffleQ, in.ShuffleO,
		in.Review.ShowScore, in.Review.ShowCorrectAnswers, in.Review.ShowExplanations,
		in.Integrity.RequireFullscreen, in.Integrity.BlockCopyPaste,
		in.Integrity.MaxFocusLoss, in.Integrity.OnLimitExceeded,
		in.Integrity.MinAwayMs, domain.Schedule.NextPublishedAt(current.publishedAt, in)); err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: update: %w", err)
	}
	if err := replaceTargets(ctx, tx, req.ID, in); err != nil {
		return domain.Assignment{}, err
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      updateAction(in, current),
		Entity:      "assignment",
		EntityID:    &req.ID,
		OccurredAt:  in.Now,
		IP:          opt.String(req.IP),
		UserAgent:   opt.String(req.UserAgent),
	}); err != nil {
		return domain.Assignment{}, err
	}

	saved, err := s.get(ctx, tx, req.ID)
	if err != nil {
		return domain.Assignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: commit update: %w", err)
	}
	return saved, nil
}

// lockedRow is what Update needs of the row it is about to overwrite.
type lockedRow struct {
	versionID             string
	closedAt, publishedAt *time.Time
}

func lockForUpdate(ctx context.Context, tx pgx.Tx, id string) (lockedRow, error) {
	var row lockedRow
	err := tx.QueryRow(ctx, `
		SELECT test_version_id::text, closed_at, published_at FROM app.assignments
		 WHERE id = $1::uuid FOR UPDATE`, id).
		Scan(&row.versionID, &row.closedAt, &row.publishedAt)
	switch {
	case err == nil:
		return row, nil
	case errors.Is(err, pgx.ErrNoRows):
		return lockedRow{}, domain.ErrNotFound
	default:
		return lockedRow{}, fmt.Errorf("assignments: load for update: %w", err)
	}
}

// replaceTargets swaps the roster wholesale: an update replaces targets rather
// than adding to them.
func replaceTargets(ctx context.Context, tx pgx.Tx, id string, in domain.WriteInput) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM app.assignment_classes WHERE assignment_id = $1::uuid`, id); err != nil {
		return fmt.Errorf("assignments: clear class targets: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM app.assignment_students WHERE assignment_id = $1::uuid`, id); err != nil {
		return fmt.Errorf("assignments: clear student targets: %w", err)
	}
	return writeTargets(ctx, tx, id, in)
}

// updateAction names the audit row: closing early and first publication are
// the two updates a teacher will be asked about later.
func updateAction(in domain.WriteInput, current lockedRow) string {
	switch {
	case in.CloseNow && current.closedAt == nil:
		return "assignment.closed"
	case current.publishedAt == nil && !in.Draft:
		return "assignment.published"
	default:
		return "assignment.updated"
	}
}

// publishedTestFor resolves the version's test and proves it is assignable.
//
// It returns the test id because app.assignments carries both, and the D-17
// composite FK rejects any pairing the caller invents.
func publishedTestFor(ctx context.Context, tx pgx.Tx, versionID string) (string, error) {
	var testID string
	err := tx.QueryRow(ctx, `
		SELECT v.test_id::text
		  FROM app.test_versions v
		  JOIN app.tests t ON t.id = v.test_id
		 WHERE v.id = $1::uuid AND t.deleted_at IS NULL AND t.status = 'published'`,
		versionID).Scan(&testID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrTestNotPublished
	}
	if err != nil {
		return "", fmt.Errorf("assignments: resolve version: %w", err)
	}
	return testID, nil
}

// checkTargets rejects ids that name nothing, rather than letting the FK fail.
//
// A foreign-key violation would surface as a 500 with no indication of which of
// forty ids was wrong.
func checkTargets(ctx context.Context, tx pgx.Tx, in domain.WriteInput) error {
	var fields []domain.FieldError

	if len(in.ClassIDs) > 0 {
		missing, err := missingIDs(ctx, tx,
			`SELECT id::text FROM app.classes WHERE id = ANY($1::uuid[])`, in.ClassIDs)
		if err != nil {
			return err
		}
		for _, id := range missing {
			fields = append(fields, domain.FieldError{Field: "targets.classIds", Message: "Không tìm thấy lớp " + id + "."})
		}
	}

	if len(in.StudentIDs) > 0 {
		missing, err := missingIDs(ctx, tx,
			`SELECT id::text FROM app.users
			  WHERE id = ANY($1::uuid[]) AND role = 'student' AND disabled_at IS NULL`,
			in.StudentIDs)
		if err != nil {
			return err
		}
		for _, id := range missing {
			fields = append(fields, domain.FieldError{Field: "targets.studentIds", Message: "Không tìm thấy học viên " + id + "."})
		}
	}

	if len(fields) > 0 {
		return &domain.ValidationError{Fields: fields}
	}
	return nil
}

func missingIDs(ctx context.Context, tx pgx.Tx, query string, want []string) ([]string, error) {
	rows, err := tx.Query(ctx, query, want)
	if err != nil {
		return nil, fmt.Errorf("assignments: check targets: %w", err)
	}
	defer rows.Close()

	found := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("assignments: check targets: %w", err)
		}
		found[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("assignments: check targets: %w", err)
	}

	var missing []string
	for _, id := range want {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

func writeTargets(ctx context.Context, tx pgx.Tx, assignmentID string, in domain.WriteInput) error {
	if len(in.ClassIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.assignment_classes (assignment_id, class_id)
			SELECT $1::uuid, unnest($2::uuid[])
			ON CONFLICT DO NOTHING`, assignmentID, in.ClassIDs); err != nil {
			return fmt.Errorf("assignments: insert class targets: %w", err)
		}
	}
	if len(in.StudentIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.assignment_students (assignment_id, user_id)
			SELECT $1::uuid, unnest($2::uuid[])
			ON CONFLICT DO NOTHING`, assignmentID, in.StudentIDs); err != nil {
			return fmt.Errorf("assignments: insert student targets: %w", err)
		}
	}
	return nil
}
