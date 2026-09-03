package assignments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrForbidden covers not targeted, not published and not found alike. Which
// assignments exist is not a student's to enumerate.
var ErrForbidden = errors.New("assignments: not this student's")

// StudentCard is what a student may know about an assignment before opening
// it: §9's card, and nothing about anyone else's work. No targets, no counts,
// no roster -- those are the teacher's projection (Assignment), and the two
// are separate types so a field added to one cannot leak through the other.
type StudentCard struct {
	ID          string
	TestTitle   string
	OpensAt     time.Time
	ClosesAt    time.Time
	ClosedAt    *time.Time
	PublishedAt *time.Time
	DurationMin int
	MaxAttempts int
	// AttemptsUsed counts finished, non-voided attempts. A live one is not
	// used yet -- it is the one the student is in.
	AttemptsUsed   int
	HasLiveAttempt bool
	// LastAttemptID is the most recent non-voided attempt, live or finished.
	LastAttemptID *string
	// Score is the last finished attempt's, and only when the assignment's
	// review policy shows scores. Nil otherwise -- absent, not zero.
	Score *Score
}

type Score struct {
	Earned        float64
	Total         float64
	PendingManual int
}

// StudentDetail is the intro page: the card plus every policy §10.2 has to
// state before the clock starts.
type StudentDetail struct {
	StudentCard
	Instructions  *string
	Review        Review
	Integrity     Integrity
	HasAudio      bool
	AudioMaxPlays *int
}

// StudentSections is §9's home, already sorted into its three lists.
type StudentSections struct {
	DueNow    []StudentCard
	Upcoming  []StudentCard
	Completed []StudentCard
}

// targeted is the roster test, written once. Both routes -- through a class
// and by name -- are checked with EXISTS rather than a join, so a student on
// both lists is one row, not two.
const targeted = `
	(EXISTS (SELECT 1 FROM app.assignment_students s
	          WHERE s.assignment_id = a.id AND s.user_id = $1::uuid)
	 OR EXISTS (SELECT 1 FROM app.assignment_classes ac
	              JOIN app.class_members m ON m.class_id = ac.class_id
	             WHERE ac.assignment_id = a.id AND m.user_id = $1::uuid))`

// The student's view of an assignment, in two halves so the intro can add its
// columns between them. `last` is their most recent non-voided attempt, live
// or finished; its score is the one submit wrote (score_earned/score_total,
// as students.go reads them), cast to float8 so numeric(8,2) lands in the wire
// type the contract promises (`format: double`).
const studentCardColumns = `
	SELECT a.id::text, t.title, a.opens_at, a.closes_at, a.closed_at, a.published_at,
	       a.duration_minutes, a.max_attempts, a.review_show_score,
	       (SELECT count(*) FROM app.attempts at
	         WHERE at.assignment_id = a.id AND at.student_id = $1::uuid
	           AND at.status IN ('submitted', 'timed_out', 'graded')),
	       EXISTS (SELECT 1 FROM app.attempts at
	                WHERE at.assignment_id = a.id AND at.student_id = $1::uuid
	                  AND at.status = 'in_progress'),
	       last.id::text, last.status::text, last.earned, last.total, last.pending`

const studentCardFrom = `
	  FROM app.assignments a
	  JOIN app.tests t ON t.id = a.test_id
	  LEFT JOIN LATERAL (
	       SELECT at.id, at.status,
	              at.score_earned::float8 AS earned,
	              coalesce(at.score_total, av.total_points)::float8 AS total,
	              (SELECT count(*) FROM app.attempt_answers ans
	                WHERE ans.attempt_id = at.id
	                  AND ans.requires_manual AND ans.manual_score IS NULL) AS pending
	         FROM app.attempts at
	         JOIN app.test_versions av ON av.id = at.test_version_id
	        WHERE at.assignment_id = a.id AND at.student_id = $1::uuid
	          AND at.status <> 'voided'
	        ORDER BY at.started_at DESC
	        LIMIT 1) last ON true`

// lastAttempt is the tail of a card row: what the LATERAL found, if anything.
type lastAttempt struct {
	id      *string
	status  *string
	earned  *float64
	total   *float64
	pending *int
}

// apply fills the card's attempt-derived fields. A score is shown only for a
// finished attempt with one recorded, and only when the assignment says so.
func (l lastAttempt) apply(c *StudentCard, showScore bool) {
	c.LastAttemptID = l.id
	finished := l.status != nil && *l.status != "in_progress"
	if showScore && finished && l.earned != nil && l.total != nil && l.pending != nil {
		c.Score = &Score{Earned: *l.earned, Total: *l.total, PendingManual: *l.pending}
	}
}

func scanStudentCard(row pgx.Row) (StudentCard, error) {
	var (
		c         StudentCard
		showScore bool
		l         lastAttempt
	)
	err := row.Scan(&c.ID, &c.TestTitle, &c.OpensAt, &c.ClosesAt, &c.ClosedAt, &c.PublishedAt,
		&c.DurationMin, &c.MaxAttempts, &showScore,
		&c.AttemptsUsed, &c.HasLiveAttempt,
		&l.id, &l.status, &l.earned, &l.total, &l.pending)
	if err != nil {
		return StudentCard{}, err
	}
	l.apply(&c, showScore)
	return c, nil
}

// ForStudent returns the home screen's three sections.
//
// Drafts never appear: publishing is what hands an assignment to students.
// Everything else is placed by one rule, in order. A live attempt is due
// whatever the window says -- it is burning a server-side clock the student
// cannot see from anywhere else, so it outranks even a nearer deadline. An
// open assignment with attempts left is due. A future one is upcoming. One
// the student has sat is completed. One that closed before they ever started
// is none of these: there is nothing to act on and nothing to show.
func (s *Store) ForStudent(ctx context.Context, studentID string, now time.Time) (StudentSections, error) {
	rows, err := s.pool.Query(ctx, studentCardColumns+studentCardFrom+`
	 WHERE a.published_at IS NOT NULL AND `+targeted+`
	 ORDER BY a.closes_at ASC, a.id DESC`, studentID)
	if err != nil {
		return StudentSections{}, fmt.Errorf("assignments: list for student: %w", err)
	}
	defer rows.Close()

	var out StudentSections
	for rows.Next() {
		c, err := scanStudentCard(rows)
		if err != nil {
			return StudentSections{}, fmt.Errorf("assignments: scan student card: %w", err)
		}
		status := StatusAt(now, c.PublishedAt, c.OpensAt, c.ClosesAt, c.ClosedAt)
		switch {
		case c.HasLiveAttempt:
			out.DueNow = append(out.DueNow, c)
		case status == Open && c.AttemptsUsed < c.MaxAttempts:
			out.DueNow = append(out.DueNow, c)
		case status == Scheduled:
			out.Upcoming = append(out.Upcoming, c)
		case c.AttemptsUsed > 0:
			out.Completed = append(out.Completed, c)
		}
	}
	return out, rows.Err()
}

// StudentDetail returns the intro for one assignment the student is targeted
// by. Not targeted, not published and not found are one answer, ErrForbidden:
// which assignments exist is not a student's to enumerate.
func (s *Store) StudentDetail(ctx context.Context, id, studentID string) (StudentDetail, error) {
	var (
		d         StudentDetail
		showScore bool
		l         lastAttempt
		onLimit   string
		maxPlays  *int
	)
	// $1 is the student, so `targeted` and the card read unchanged; the
	// assignment is $2. Instructions stay nil: the schema keeps them per
	// section (test_version_sections.instructions), and the contract's
	// test-level field has no column to read from.
	err := s.pool.QueryRow(ctx, studentCardColumns+`,
	       a.review_show_correct_answers, a.review_show_explanations,
	       a.integrity_require_fullscreen, a.integrity_block_copy_paste,
	       a.integrity_max_focus_loss, a.integrity_on_limit_exceeded::text,
	       a.integrity_min_away_ms,
	       EXISTS (SELECT 1 FROM app.test_version_questions q
	                 JOIN app.test_version_sections sec ON sec.id = q.test_version_section_id
	                WHERE sec.test_version_id = a.test_version_id
	                  AND q.media_asset_kind = 'audio'),
	       -- The strictest cap on the paper. NULL when every listening question
	       -- is unlimited, which the intro reads as "no limit to state".
	       (SELECT min(q.audio_max_plays) FROM app.test_version_questions q
	          JOIN app.test_version_sections sec ON sec.id = q.test_version_section_id
	         WHERE sec.test_version_id = a.test_version_id
	           AND q.media_asset_kind = 'audio')`+studentCardFrom+`
	 WHERE a.id = $2::uuid AND a.published_at IS NOT NULL AND `+targeted,
		studentID, id).Scan(
		&d.ID, &d.TestTitle, &d.OpensAt, &d.ClosesAt, &d.ClosedAt, &d.PublishedAt,
		&d.DurationMin, &d.MaxAttempts, &showScore,
		&d.AttemptsUsed, &d.HasLiveAttempt,
		&l.id, &l.status, &l.earned, &l.total, &l.pending,
		&d.Review.ShowCorrectAnswers, &d.Review.ShowExplanations,
		&d.Integrity.RequireFullscreen, &d.Integrity.BlockCopyPaste,
		&d.Integrity.MaxFocusLoss, &onLimit, &d.Integrity.MinAwayMs,
		&d.HasAudio, &maxPlays)
	if errors.Is(err, pgx.ErrNoRows) {
		return StudentDetail{}, ErrForbidden
	}
	if err != nil {
		return StudentDetail{}, fmt.Errorf("assignments: student detail: %w", err)
	}
	d.Review.ShowScore = showScore
	d.Integrity.OnLimitExceeded = onLimit
	l.apply(&d.StudentCard, showScore)
	if d.HasAudio {
		d.AudioMaxPlays = maxPlays
	}
	return d, nil
}
