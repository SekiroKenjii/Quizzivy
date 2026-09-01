package attempts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

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
			// No row, nothing to update, and zero either way. An unanswered
			// short_answer is not work waiting for the teacher -- there is
			// nothing to read.
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

	// §7: the total comes from the frozen version, not from summing the
	// questions here. A paper's worth is decided when it is published.
	var total float64
	if err := tx.QueryRow(ctx,
		`SELECT total_points FROM app.test_versions WHERE id = $1::uuid`, versionID).
		Scan(&total); err != nil {
		return row{}, fmt.Errorf("attempts: read total points: %w", err)
	}

	// Ended-at is the deadline for a timeout and now for anything else: the
	// work stopped when the clock did, whether or not anyone was watching.
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
//
// An earlier version jumped straight to `graded` when nothing needed a person,
// which quietly destroyed the more interesting fact: a paper that ran out of
// time and happened to contain no essay read back as `graded`, and nothing
// anywhere said the student never finished it.
//
// `graded` is the teacher's to set, when they finish grading (§8). Nothing here
// sets it, and a paper with no essay does not sit in anyone's queue as a
// result: the queue counts ANSWERS awaiting a person
// (requires_manual AND manual_score IS NULL), not attempts in a status.
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
	rows, err := tx.Query(ctx, `
		SELECT o.test_version_question_id, o.id, o.ordinal, o.is_correct
		  FROM app.test_version_options o
		  JOIN app.test_version_questions q ON q.id = o.test_version_question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1::uuid`, versionID)
	if err != nil {
		return fmt.Errorf("attempts: read key options: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var questionID string
		var o grading.Option
		if err := rows.Scan(&questionID, &o.ID, &o.Ordinal, &o.Correct); err != nil {
			return fmt.Errorf("attempts: scan key option: %w", err)
		}
		if i, ok := at[questionID]; ok {
			qs[i].Options = append(qs[i].Options, o)
		}
	}
	return rows.Err()
}

func attachKeyBlanks(ctx context.Context, tx pgx.Tx, versionID string, qs []grading.Question, at map[string]int) error {
	rows, err := tx.Query(ctx, `
		SELECT b.test_version_question_id, b.id, b.case_sensitive,
		       coalesce(array_agg(ba.answer) FILTER (WHERE ba.answer IS NOT NULL), '{}')
		  FROM app.test_version_blanks b
		  JOIN app.test_version_questions q ON q.id = b.test_version_question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		  LEFT JOIN app.test_version_blank_answers ba ON ba.test_version_blank_id = b.id
		 WHERE s.test_version_id = $1::uuid
		 GROUP BY b.test_version_question_id, b.id, b.case_sensitive`, versionID)
	if err != nil {
		return fmt.Errorf("attempts: read key blanks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var questionID string
		var b grading.Blank
		if err := rows.Scan(&questionID, &b.ID, &b.CaseSensitive, &b.Accepted); err != nil {
			return fmt.Errorf("attempts: scan key blank: %w", err)
		}
		if i, ok := at[questionID]; ok {
			qs[i].Blanks = append(qs[i].Blanks, b)
		}
	}
	return rows.Err()
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
//
// This is what makes a deadline take effect with nothing watching the clock
// (D-18's argument, applied to attempts rather than assignments). A scheduler
// would be a second moving part that has to be running, be monitored, and be
// correct about a timezone; a read that already had to happen is neither.
//
// The cost is that an attempt nobody looks at stays in_progress in the table
// until somebody does. Nothing reads status without going through here, so the
// row is never observed in the stale state -- which is the only property that
// matters.
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
