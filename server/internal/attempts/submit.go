package attempts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/db"
	"quizzivy/internal/grading"
)

// Reason records how an attempt ended. It is the contract's `reason`, kept out
// of the status column: status says what still has to happen to the attempt,
// this says what stopped it.
type Reason string

const (
	Manual       Reason = "manual"
	TimerExpired Reason = "timer_expired"
	AutoSubmit   Reason = "auto_submit"
)

// Submit closes an attempt and grades everything a machine can.
//
// Idempotent by the lock plus the status check: a manual tap racing the timer's
// auto-submit means one of them takes the row and the other reads a status that
// is no longer in_progress. That is 409 ATTEMPT_CLOSED, not a second grading
// pass over the same answers.
func (s *Store) Submit(ctx context.Context, attemptID, studentID string, reason Reason, now time.Time) (row, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return row{}, fmt.Errorf("attempts: begin submit: %w", err)
	}
	defer tx.Rollback(ctx)

	var (
		status     Status
		versionID  string
		deadlineAt time.Time
	)
	err = tx.QueryRow(ctx, `
		SELECT status, test_version_id, deadline_at
		  FROM app.attempts
		 WHERE id = $1::uuid AND ($2 = '' OR student_id = $2::uuid)
		   FOR UPDATE`, attemptID, studentID).Scan(&status, &versionID, &deadlineAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return row{}, ErrForbidden
	}
	if err != nil {
		return row{}, fmt.Errorf("attempts: lock attempt for submit: %w", err)
	}
	if status != InProgress {
		return row{}, ErrAttemptClosed
	}

	closed, err := gradeAndClose(ctx, tx, attemptID, versionID, reason, deadlineAt, now)
	if err != nil {
		return row{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return row{}, fmt.Errorf("attempts: commit submit: %w", err)
	}
	return closed, nil
}

func gradeAndClose(ctx context.Context, tx pgx.Tx, attemptID, versionID string, reason Reason, deadlineAt, now time.Time) (row, error) {
	questions, err := gradingKey(ctx, tx, versionID)
	if err != nil {
		return row{}, err
	}
	answers, err := submittedAnswers(ctx, tx, attemptID)
	if err != nil {
		return row{}, err
	}

	var (
		earned float64
		ids    []string
		scores []float64
		manual []bool
	)
	for _, q := range questions {
		payload, answered := answers[q.ID]
		if !answered {
			// No row, nothing to update, and zero either way.
			continue
		}
		result := grading.Grade(q, payload)
		earned += result.Score
		ids = append(ids, q.ID)
		scores = append(scores, result.Score)
		manual = append(manual, result.RequiresManual)
	}

	if len(ids) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE app.attempt_answers a
			   SET auto_score = graded.score, requires_manual = graded.manual
			  FROM unnest($2::uuid[], $3::numeric[], $4::boolean[])
			         AS graded(question_id, score, manual)
			 WHERE a.attempt_id = $1::uuid AND a.question_id = graded.question_id`,
			attemptID, ids, scores, manual); err != nil {
			return row{}, fmt.Errorf("attempts: write auto scores: %w", err)
		}
	}

	// §7: the total comes from the frozen version, not from summing the questions here.
	var total float64
	if err := tx.QueryRow(ctx,
		`SELECT total_points FROM app.test_versions WHERE id = $1::uuid`, versionID).
		Scan(&total); err != nil {
		return row{}, fmt.Errorf("attempts: read total points: %w", err)
	}

	endedAt := now
	if reason == TimerExpired {
		endedAt = deadlineAt
	}

	closed, err := scanAttempt(tx.QueryRow(ctx, `
		UPDATE app.attempts
		   SET status = $2::app.attempt_status,
		       submitted_at = $3::timestamptz,
		       score_earned = $4, score_total = $5,
		       graded_at = CASE WHEN $2::app.attempt_status = 'graded'
		                        THEN $3::timestamptz END
		 WHERE id = $1::uuid
		RETURNING `+attemptColumns,
		attemptID, string(closingStatus(reason)), endedAt, earned, total))
	if err != nil {
		return row{}, fmt.Errorf("attempts: close attempt: %w", err)
	}
	return closed, nil
}

// closingStatus records HOW the attempt ended, and nothing about grading.
func closingStatus(reason Reason) Status {
	if reason == TimerExpired {
		return TimedOut
	}
	return Submitted
}

// gradingKey reads the answers. This is the one place in the package that does,
// and it is reachable only from Submit -- the student payload has no field to
// put any of it in (§13.5).
func gradingKey(ctx context.Context, tx pgx.Tx, versionID string) ([]grading.Question, error) {
	rows, err := tx.Query(ctx, `
		SELECT q.id, q.type, q.points
		  FROM app.test_version_questions q
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1::uuid`, versionID)
	if err != nil {
		return nil, fmt.Errorf("attempts: read grading key: %w", err)
	}
	defer rows.Close()

	var questions []grading.Question
	at := map[string]int{}
	for rows.Next() {
		var q grading.Question
		if err := rows.Scan(&q.ID, &q.Type, &q.Points); err != nil {
			return nil, fmt.Errorf("attempts: scan grading key: %w", err)
		}
		at[q.ID] = len(questions)
		questions = append(questions, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("attempts: read grading key: %w", err)
	}

	if err := attachKeyOptions(ctx, tx, versionID, questions, at); err != nil {
		return nil, err
	}
	return questions, attachKeyBlanks(ctx, tx, versionID, questions, at)
}

func attachKeyOptions(ctx context.Context, tx pgx.Tx, versionID string, qs []grading.Question, at map[string]int) error {
	byQuestion, err := db.GroupBy(ctx, tx, `
		SELECT o.test_version_question_id, o.id, o.ordinal, o.is_correct
		  FROM app.test_version_options o
		  JOIN app.test_version_questions q ON q.id = o.test_version_question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1::uuid`, []any{versionID},
		func(rows pgx.Rows) (string, grading.Option, error) {
			var questionID string
			var o grading.Option
			err := rows.Scan(&questionID, &o.ID, &o.Ordinal, &o.Correct)
			return questionID, o, err
		})
	if err != nil {
		return fmt.Errorf("attempts: read key options: %w", err)
	}
	for questionID, options := range byQuestion {
		if i, ok := at[questionID]; ok {
			qs[i].Options = options
		}
	}
	return nil
}

func attachKeyBlanks(ctx context.Context, tx pgx.Tx, versionID string, qs []grading.Question, at map[string]int) error {
	byQuestion, err := db.GroupBy(ctx, tx, `
		SELECT b.test_version_question_id, b.id, b.case_sensitive,
		       coalesce(array_agg(ba.answer) FILTER (WHERE ba.answer IS NOT NULL), '{}')
		  FROM app.test_version_blanks b
		  JOIN app.test_version_questions q ON q.id = b.test_version_question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		  LEFT JOIN app.test_version_blank_answers ba ON ba.test_version_blank_id = b.id
		 WHERE s.test_version_id = $1::uuid
		 GROUP BY b.test_version_question_id, b.id, b.case_sensitive`, []any{versionID},
		func(rows pgx.Rows) (string, grading.Blank, error) {
			var questionID string
			var b grading.Blank
			err := rows.Scan(&questionID, &b.ID, &b.CaseSensitive, &b.Accepted)
			return questionID, b, err
		})
	if err != nil {
		return fmt.Errorf("attempts: read key blanks: %w", err)
	}
	for questionID, blanks := range byQuestion {
		if i, ok := at[questionID]; ok {
			qs[i].Blanks = blanks
		}
	}
	return nil
}

func submittedAnswers(ctx context.Context, tx pgx.Tx, attemptID string) (map[string][]byte, error) {
	rows, err := tx.Query(ctx, `
		SELECT question_id, payload FROM app.attempt_answers
		 WHERE attempt_id = $1::uuid`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("attempts: read answers for grading: %w", err)
	}
	defer rows.Close()

	out := map[string][]byte{}
	for rows.Next() {
		var questionID string
		var payload []byte
		if err := rows.Scan(&questionID, &payload); err != nil {
			return nil, fmt.Errorf("attempts: scan answer for grading: %w", err)
		}
		out[questionID] = payload
	}
	return out, rows.Err()
}

func (s *Service) Submit(ctx context.Context, attemptID, studentID string, reason Reason) (Attempt, error) {
	closed, err := s.store.Submit(ctx, attemptID, studentID, reason, s.now())
	return closed.Attempt, err
}

// ExpireIfDue closes an attempt whose time ran out, and does nothing to one
// that has not.
func (s *Store) ExpireIfDue(ctx context.Context, attemptID string, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("attempts: begin expiry: %w", err)
	}
	defer tx.Rollback(ctx)

	var (
		status     Status
		versionID  string
		deadlineAt time.Time
	)
	err = tx.QueryRow(ctx, `
		SELECT status, test_version_id, deadline_at
		  FROM app.attempts WHERE id = $1::uuid FOR UPDATE`, attemptID).
		Scan(&status, &versionID, &deadlineAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("attempts: lock attempt for expiry: %w", err)
	}
	if status != InProgress || !now.After(deadlineAt) {
		return nil
	}

	if _, err := gradeAndClose(ctx, tx, attemptID, versionID, TimerExpired, deadlineAt, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
