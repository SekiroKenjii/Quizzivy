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
	ID        string
	TestTitle string
	// ClassName is set only when exactly one targeted class contains them.
	ClassName     *string
	OpensAt       time.Time
	ClosesAt      time.Time
	ClosedAt      *time.Time
	PublishedAt   *time.Time
	DurationMin   int
	MaxAttempts   int
	QuestionCount int
	TotalPoints   float64
	AttemptsUsed  int
	// HasLiveAttempt means resumable: in progress and before its deadline.
	HasLiveAttempt bool
	// LiveDeadlineAt is non-nil exactly when HasLiveAttempt is true.
	LiveDeadlineAt *time.Time
	// LastAttemptID is the most recent non-voided attempt, live or finished.
	LastAttemptID *string
	// LastSubmittedAt is nil while that attempt is still live.
	LastSubmittedAt *time.Time
	Score           *Score
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
	// TeacherName is the assignment's author.
	TeacherName *string
	Review      Review
	Integrity   Integrity
	HasAudio    bool
	// ShowsTranscript is true when any listening question releases one.
	ShowsTranscript bool
	AudioMaxPlays   *int
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
//
// Disabled accounts are excluded here because bearer verification is pure, so
// a token outlives the account by its TTL.
const targeted = `
	(EXISTS (SELECT 1 FROM app.users me
	          WHERE me.id = $1::uuid AND me.disabled_at IS NULL)
	 AND (EXISTS (SELECT 1 FROM app.assignment_students s
	               WHERE s.assignment_id = a.id AND s.user_id = $1::uuid)
	      OR EXISTS (SELECT 1 FROM app.assignment_classes ac
	                   JOIN app.class_members m ON m.class_id = ac.class_id
	                  WHERE ac.assignment_id = a.id AND m.user_id = $1::uuid)))`

// The student's view of an assignment, in two halves so the intro can add its
// columns between them. `last` is their most recent non-voided attempt, live
// or finished; its score is the one submit wrote (score_earned/score_total,
// as students.go reads them), cast to float8 so numeric(8,2) lands in the wire
// type the contract promises (`format: double`).
const studentCardColumns = `
	SELECT a.id::text, t.title,
	       (SELECT CASE WHEN count(*) = 1 THEN min(c.name) END
	          FROM app.assignment_classes ac
	          JOIN app.classes c ON c.id = ac.class_id
	          JOIN app.class_members m ON m.class_id = ac.class_id
	                                  AND m.user_id = $1::uuid
	         WHERE ac.assignment_id = a.id),
	       a.opens_at, a.closes_at, a.closed_at, a.published_at,
	       a.duration_minutes, a.max_attempts, a.review_show_score,
	       (SELECT count(*) FROM app.test_version_questions q
	          JOIN app.test_version_sections sec ON sec.id = q.test_version_section_id
	         WHERE sec.test_version_id = a.test_version_id),
	       v.total_points::float8,
	       -- "Live" means resumable, the way resumeIfLive means it: in progress
	       -- AND before its deadline. One left open past the deadline in a
	       -- closed tab is spent -- the server times it out at the next contact
	       -- -- so it counts as used here rather than offering a resume the
	       -- server would refuse.
	       (SELECT count(*) FROM app.attempts at
	         WHERE at.assignment_id = a.id AND at.student_id = $1::uuid
	           AND (at.status IN ('submitted', 'timed_out', 'graded')
	                OR (at.status = 'in_progress' AND at.deadline_at <= now()))),
	       EXISTS (SELECT 1 FROM app.attempts at
	                WHERE at.assignment_id = a.id AND at.student_id = $1::uuid
	                  AND at.status = 'in_progress' AND at.deadline_at > now()),
	       -- The same WHERE as the EXISTS above, so the two cannot disagree.
	       (SELECT at.deadline_at FROM app.attempts at
	         WHERE at.assignment_id = a.id AND at.student_id = $1::uuid
	           AND at.status = 'in_progress' AND at.deadline_at > now()
	         ORDER BY at.deadline_at DESC LIMIT 1),
	       last.id::text, last.status::text, last.submitted_at,
	       last.earned, last.total, last.pending`

const studentCardFrom = `
	  FROM app.assignments a
	  JOIN app.tests t ON t.id = a.test_id
	  JOIN app.test_versions v ON v.id = a.test_version_id
	  LEFT JOIN LATERAL (
	       SELECT at.id, at.status, at.submitted_at,
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
	id          *string
	status      *string
	submittedAt *time.Time
	earned      *float64
	total       *float64
	pending     *int
}

// apply fills the card's attempt-derived fields. A score is shown only for a
// finished attempt with one recorded, and only when the assignment says so.
func (l lastAttempt) apply(c *StudentCard, showScore bool) {
	c.LastAttemptID = l.id
	c.LastSubmittedAt = l.submittedAt
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
	err := row.Scan(&c.ID, &c.TestTitle, &c.ClassName,
		&c.OpensAt, &c.ClosesAt, &c.ClosedAt, &c.PublishedAt,
		&c.DurationMin, &c.MaxAttempts, &showScore,
		&c.QuestionCount, &c.TotalPoints,
		&c.AttemptsUsed, &c.HasLiveAttempt, &c.LiveDeadlineAt,
		&l.id, &l.status, &l.submittedAt, &l.earned, &l.total, &l.pending)
	if err != nil {
		return StudentCard{}, err
	}
	l.apply(&c, showScore)
	return c, nil
}

// ForStudent returns the home screen's three sections.
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
	// $1 is the student, so `targeted` and the card read unchanged; $2 is the assignment.
	err := s.pool.QueryRow(ctx, studentCardColumns+`,
	       (SELECT au.full_name FROM app.users au WHERE au.id = a.created_by),
	       a.review_show_correct_answers, a.review_show_explanations,
	       a.integrity_require_fullscreen, a.integrity_block_copy_paste,
	       a.integrity_max_focus_loss, a.integrity_on_limit_exceeded::text,
	       a.integrity_min_away_ms,
	       EXISTS (SELECT 1 FROM app.test_version_questions q
	                 JOIN app.test_version_sections sec ON sec.id = q.test_version_section_id
	                WHERE sec.test_version_id = a.test_version_id
	                  AND q.media_asset_kind = 'audio'),
	       EXISTS (SELECT 1 FROM app.test_version_questions q
	                 JOIN app.test_version_sections sec ON sec.id = q.test_version_section_id
	                WHERE sec.test_version_id = a.test_version_id
	                  AND q.audio_show_transcript_after),
	       -- The strictest cap on the paper. NULL when every listening question
	       -- is unlimited, which the intro reads as "no limit to state".
	       (SELECT min(q.audio_max_plays) FROM app.test_version_questions q
	          JOIN app.test_version_sections sec ON sec.id = q.test_version_section_id
	         WHERE sec.test_version_id = a.test_version_id
	           AND q.media_asset_kind = 'audio')`+studentCardFrom+`
	 WHERE a.id = $2::uuid AND a.published_at IS NOT NULL AND `+targeted,
		studentID, id).Scan(
		&d.ID, &d.TestTitle, &d.ClassName,
		&d.OpensAt, &d.ClosesAt, &d.ClosedAt, &d.PublishedAt,
		&d.DurationMin, &d.MaxAttempts, &showScore,
		&d.QuestionCount, &d.TotalPoints,
		&d.AttemptsUsed, &d.HasLiveAttempt, &d.LiveDeadlineAt,
		&l.id, &l.status, &l.submittedAt, &l.earned, &l.total, &l.pending,
		&d.TeacherName,
		&d.Review.ShowCorrectAnswers, &d.Review.ShowExplanations,
		&d.Integrity.RequireFullscreen, &d.Integrity.BlockCopyPaste,
		&d.Integrity.MaxFocusLoss, &onLimit, &d.Integrity.MinAwayMs,
		&d.HasAudio, &d.ShowsTranscript, &maxPlays)
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
