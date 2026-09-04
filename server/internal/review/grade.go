package review

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/attempts"
)

// Item is one manual mark: the question, the points, and a comment the
// student will read.
type Item struct {
	QuestionID string
	Points     float64
	Comment    *string
}

// ItemError names an item that could not be marked and why -- `not_on_paper`,
// `unanswered` or `above_ceiling`.
type ItemError struct {
	QuestionID string
	Reason     string
}

type ValidationError struct{ Items []ItemError }

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Items))
	for i, it := range e.Items {
		parts[i] = it.QuestionID + ": " + it.Reason
	}
	return "review: " + strings.Join(parts, "; ")
}

// Grade writes manual marks for a closed attempt and returns the live score.
//
// Saved per call rather than as one submit, so a half-graded paper survives a
// refresh (§8). `points` above the question's ceiling is refused; a question
// the student never answered has no row to mark and is refused too, because
// a mark with no work behind it is a number nobody can explain later.
func (s *Store) Grade(ctx context.Context, attemptID, graderID string, items []Item) (attempts.Score, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return attempts.Score{}, fmt.Errorf("review: begin grade: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	versionID, err := lockGradable(ctx, tx, attemptID)
	if err != nil {
		return attempts.Score{}, err
	}
	if err := validate(ctx, tx, attemptID, versionID, items); err != nil {
		return attempts.Score{}, err
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
		return attempts.Score{}, fmt.Errorf("review: write marks: %w", err)
	}

	score, err := recomputeScore(ctx, tx, attemptID)
	if err != nil {
		return attempts.Score{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return attempts.Score{}, fmt.Errorf("review: commit grade: %w", err)
	}
	return score, nil
}

// Finish declares the paper graded. Re-enterable: a graded attempt can be
// marked again and finished again, and the score is recomputed each time.
func (s *Store) Finish(ctx context.Context, attemptID string) (attempts.Attempt, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return attempts.Attempt{}, fmt.Errorf("review: begin finish: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := lockGradable(ctx, tx, attemptID); err != nil {
		return attempts.Attempt{}, err
	}
	score, err := recomputeScore(ctx, tx, attemptID)
	if err != nil {
		return attempts.Attempt{}, err
	}
	if score.PendingManual > 0 {
		return attempts.Attempt{}, ErrIncomplete
	}

	var a attempts.Attempt
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
		return attempts.Attempt{}, fmt.Errorf("review: finish: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return attempts.Attempt{}, fmt.Errorf("review: commit finish: %w", err)
	}
	return a, nil
}

// lockGradable takes the row lock and refuses the two states grading cannot
// apply to: nothing handed in yet, or nothing that counts.
func lockGradable(ctx context.Context, tx pgx.Tx, attemptID string) (string, error) {
	var status attempts.Status
	var versionID string
	err := tx.QueryRow(ctx, `
		SELECT status, test_version_id::text FROM app.attempts
		 WHERE id = $1::uuid FOR UPDATE`, attemptID).Scan(&status, &versionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("review: lock attempt: %w", err)
	}
	switch status {
	case attempts.InProgress:
		return "", ErrInProgress
	case attempts.Voided:
		return "", ErrVoided
	}
	return versionID, nil
}

func validate(ctx context.Context, tx pgx.Tx, attemptID, versionID string, items []Item) error {
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

	var invalid []ItemError
	for _, it := range items {
		sl, ok := paper[it.QuestionID]
		switch {
		case !ok:
			invalid = append(invalid, ItemError{it.QuestionID, "not_on_paper"})
		case !sl.answered:
			invalid = append(invalid, ItemError{it.QuestionID, "unanswered"})
		case it.Points > sl.ceiling:
			invalid = append(invalid, ItemError{it.QuestionID, "above_ceiling"})
		}
	}
	if len(invalid) > 0 {
		return &ValidationError{Items: invalid}
	}
	return nil
}

// recomputeScore folds `final_score` -- the VIRTUAL column, so manual
// precedence over auto cannot drift (§13.3) -- back onto the attempt.
func recomputeScore(ctx context.Context, tx pgx.Tx, attemptID string) (attempts.Score, error) {
	var out attempts.Score
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
		return attempts.Score{}, fmt.Errorf("review: recompute score: %w", err)
	}
	if total != nil {
		out.Total = *total
	}
	return out, nil
}
