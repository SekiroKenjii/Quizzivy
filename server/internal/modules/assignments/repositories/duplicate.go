package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/visibility"
	"time"

	"github.com/jackc/pgx/v5"
)

const duplicatedClassField = "classIds"

// Duplicate copies an assignment the actor reaches as a draft owned by the
// actor: the same version, time limit, attempts, shuffle flags, review options,
// integrity policy and note to students, a window that opens at now and lasts
// as long as the original's, and the classes named as its only targets. The
// version must be one the actor may assign, as for Create. Another teacher's
// assignment answers ErrNotFound, exactly as a missing one does, and a class
// the actor does not teach answers as an unknown one does, on classIds.
func (s *Postgres) Duplicate(ctx context.Context, req domain.Request, classIDs []string, now time.Time) (domain.Assignment, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: begin duplicate: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var versionID string
	err = tx.QueryRow(ctx, `
		SELECT test_version_id::text FROM app.assignments
		 WHERE id = $1::uuid AND ($2::boolean OR id IN `+visibility.AssignmentIDs(3)+`)`,
		req.ID, req.All, opt.String(req.ActorID)).Scan(&versionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Assignment{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: read original: %w", err)
	}
	if _, err := publishedTestFor(ctx, tx, req.Scope(), versionID); err != nil {
		return domain.Assignment{}, err
	}
	targets := domain.WriteInput{ClassIDs: classIDs}
	if err := checkTargets(ctx, tx, req.Scope(), "", targets); err != nil {
		return domain.Assignment{}, renamedTargetFields(err)
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO app.assignments
		       (test_id, test_version_id, opens_at, closes_at, closed_at,
		        duration_minutes, max_attempts, shuffle_questions, shuffle_options,
		        review_show_score, review_show_correct_answers, review_show_explanations,
		        review_release, review_show_class_average, student_note,
		        integrity_require_fullscreen, integrity_block_copy_paste,
		        integrity_max_focus_loss, integrity_on_limit_exceeded, integrity_min_away_ms,
		        created_by, published_at)
		SELECT o.test_id, o.test_version_id, $2::timestamptz, $2::timestamptz + (o.closes_at - o.opens_at), NULL,
		       o.duration_minutes, o.max_attempts, o.shuffle_questions, o.shuffle_options,
		       o.review_show_score, o.review_show_correct_answers, o.review_show_explanations,
		       o.review_release, o.review_show_class_average, o.student_note,
		       o.integrity_require_fullscreen, o.integrity_block_copy_paste,
		       o.integrity_max_focus_loss, o.integrity_on_limit_exceeded, o.integrity_min_away_ms,
		       $3::uuid, NULL
		  FROM app.assignments o
		 WHERE o.id = $1::uuid AND ($4::boolean OR o.id IN `+visibility.AssignmentIDs(3)+`)
		RETURNING id::text`,
		req.ID, now, req.ActorID, req.All).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Assignment{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: insert copy: %w", err)
	}

	if err := writeTargets(ctx, tx, id, targets); err != nil {
		return domain.Assignment{}, err
	}
	diff, err := json.Marshal(map[string]string{"sourceId": req.ID})
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: encode duplicate audit: %w", err)
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "assignment.duplicated",
		Entity:      entityAssignment,
		EntityID:    &id,
		OccurredAt:  now,
		IP:          opt.String(req.IP),
		UserAgent:   opt.String(req.UserAgent),
		Diff:        diff,
	}); err != nil {
		return domain.Assignment{}, err
	}

	copied, err := s.get(ctx, tx, req.Scope(), id, false)
	if err != nil {
		return domain.Assignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: commit duplicate: %w", err)
	}
	return copied, nil
}

func renamedTargetFields(err error) error {
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) {
		return err
	}
	fields := make([]domain.FieldError, len(invalid.Fields))
	for i, f := range invalid.Fields {
		fields[i] = f
		fields[i].Field = duplicatedClassField
	}
	return &domain.ValidationError{Fields: fields}
}
