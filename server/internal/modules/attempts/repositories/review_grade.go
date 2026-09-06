package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"

	"github.com/jackc/pgx/v5"
)

// Grade writes manual marks for a closed attempt and returns the live score.
func (s *Reviews) Grade(ctx context.Context, attemptID, graderID string, items []domain.GradeItem) (domain.Score, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Score{}, fmt.Errorf("review: begin grade: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	versionID, err := lockGradable(ctx, tx, attemptID)
	if err != nil {
		return domain.Score{}, err
	}
	if err := validateGrades(ctx, tx, attemptID, versionID, items); err != nil {
		return domain.Score{}, err
	}

	ids := make([]string, len(items))
	points := make([]float64, len(items))
	comments := make([]*string, len(items))
	for i, it := range items {
		ids[i] = it.QuestionID
		points[i] = it.Points
		comments[i] = it.Comment
	}
	if _, err := tx.Exec(ctx, `
		UPDATE app.attempt_answers a
		   SET manual_score = marked.points, grader_comment = marked.comment,
		       graded_by = $2::uuid, graded_at = $3
		  FROM unnest($4::uuid[], $5::numeric[], $6::text[]) AS marked(question_id, points, comment)
		 WHERE a.attempt_id = $1::uuid AND a.question_id = marked.question_id`,
		attemptID, graderID, s.now(), ids, points, comments); err != nil {
		return domain.Score{}, fmt.Errorf("review: write marks: %w", err)
	}

	score, err := recomputeScore(ctx, tx, attemptID)
	if err != nil {
		return domain.Score{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Score{}, fmt.Errorf("review: commit grade: %w", err)
	}
	return score, nil
}

// Finish declares the paper graded. Re-enterable: a graded attempt can be
// marked again and finished again, and the score is recomputed each time.
func (s *Reviews) Finish(ctx context.Context, attemptID string) (domain.Attempt, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Attempt{}, fmt.Errorf("review: begin finish: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := lockGradable(ctx, tx, attemptID); err != nil {
		return domain.Attempt{}, err
	}
	score, err := recomputeScore(ctx, tx, attemptID)
	if err != nil {
		return domain.Attempt{}, err
	}
	if score.PendingManual > 0 {
		return domain.Attempt{}, domain.ErrGradingIncomplete
	}

	var a domain.Attempt
	if err := tx.QueryRow(ctx, `
		UPDATE app.attempts
		   SET status = 'graded', graded_at = $2
		 WHERE id = $1::uuid
		RETURNING id::text, assignment_id::text, student_id::text, test_version_id::text,
		          attempt_no, status, started_at, deadline_at, submitted_at, graded_at,
		          focus_loss_count, flagged`, attemptID, s.now()).Scan(
		&a.ID, &a.AssignmentID, &a.StudentID, &a.TestVersionID,
		&a.AttemptNo, &a.Status, &a.StartedAt, &a.DeadlineAt, &a.SubmittedAt, &a.GradedAt,
		&a.FocusLossCount, &a.Flagged); err != nil {
		return domain.Attempt{}, fmt.Errorf("review: finish: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Attempt{}, fmt.Errorf("review: commit finish: %w", err)
	}
	return a, nil
}

func lockGradable(ctx context.Context, tx pgx.Tx, attemptID string) (string, error) {
	var status domain.Status
	var versionID string
	err := tx.QueryRow(ctx, `
		SELECT status, test_version_id::text FROM app.attempts
		 WHERE id = $1::uuid FOR UPDATE`, attemptID).Scan(&status, &versionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrPaperNotFound
	}
	if err != nil {
		return "", fmt.Errorf("review: lock attempt: %w", err)
	}
	switch status {
	case domain.InProgress:
		return "", domain.ErrPaperInProgress
	case domain.Voided:
		return "", domain.ErrPaperVoided
	}
	return versionID, nil
}

func validateGrades(ctx context.Context, tx pgx.Tx, attemptID, versionID string, items []domain.GradeItem) error {
	rows, err := tx.Query(ctx, `
		SELECT q.id::text, q.points, (aa.attempt_id IS NOT NULL)
		  FROM app.test_version_questions q
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		  LEFT JOIN app.attempt_answers aa
		    ON aa.attempt_id = $2::uuid AND aa.question_id = q.id
		 WHERE s.test_version_id = $1::uuid`, versionID, attemptID)
	if err != nil {
		return fmt.Errorf("review: read paper for grading: %w", err)
	}
	defer rows.Close()

	type slot struct {
		ceiling  float64
		answered bool
	}
	paper := map[string]slot{}
	for rows.Next() {
		var id string
		var sl slot
		if err := rows.Scan(&id, &sl.ceiling, &sl.answered); err != nil {
			return fmt.Errorf("review: scan paper: %w", err)
		}
		paper[id] = sl
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("review: read paper for grading: %w", err)
	}

	var invalid []domain.GradeItemError
	for _, it := range items {
		sl, ok := paper[it.QuestionID]
		switch {
		case !ok:
			invalid = append(invalid, domain.GradeItemError{QuestionID: it.QuestionID, Reason: "not_on_paper"})
		case !sl.answered:
			invalid = append(invalid, domain.GradeItemError{QuestionID: it.QuestionID, Reason: "unanswered"})
		case it.Points > sl.ceiling:
			invalid = append(invalid, domain.GradeItemError{QuestionID: it.QuestionID, Reason: "above_ceiling"})
		}
	}
	if len(invalid) > 0 {
		return &domain.GradeValidationError{Items: invalid}
	}
	return nil
}

func recomputeScore(ctx context.Context, tx pgx.Tx, attemptID string) (domain.Score, error) {
	var out domain.Score
	var total *float64
	err := tx.QueryRow(ctx, `
		UPDATE app.attempts at
		   SET score_earned = (SELECT coalesce(sum(aa.final_score), 0)
		                         FROM app.attempt_answers aa WHERE aa.attempt_id = at.id)
		 WHERE at.id = $1::uuid
		RETURNING at.score_earned, at.score_total,
		          (SELECT count(*) FROM app.attempt_answers aa
		            WHERE aa.attempt_id = at.id AND aa.requires_manual AND aa.manual_score IS NULL)`,
		attemptID).Scan(&out.Earned, &total, &out.PendingManual)
	if err != nil {
		return domain.Score{}, fmt.Errorf("review: recompute score: %w", err)
	}
	if total != nil {
		out.Total = *total
	}
	return out, nil
}
