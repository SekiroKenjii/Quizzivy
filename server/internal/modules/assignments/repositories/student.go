package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/assignments/domain"
	"time"

	"github.com/jackc/pgx/v5"
)

const targeted = `
	(EXISTS (SELECT 1 FROM app.users me
	          WHERE me.id = $1::uuid AND me.disabled_at IS NULL)
	 AND (EXISTS (SELECT 1 FROM app.assignment_students s
	               WHERE s.assignment_id = a.id AND s.user_id = $1::uuid)
	      OR EXISTS (SELECT 1 FROM app.assignment_classes ac
	                   JOIN app.class_members m ON m.class_id = ac.class_id
	                  WHERE ac.assignment_id = a.id AND m.user_id = $1::uuid)))`

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

type lastAttempt struct {
	id          *string
	status      *string
	submittedAt *time.Time
	earned      *float64
	total       *float64
	pending     *int
}

func (l lastAttempt) apply(c *domain.StudentCard, showScore bool) {
	c.LastAttemptID = l.id
	c.LastSubmittedAt = l.submittedAt
	finished := l.status != nil && *l.status != "in_progress"
	if showScore && finished && l.earned != nil && l.total != nil && l.pending != nil {
		c.Score = &domain.Score{Earned: *l.earned, Total: *l.total, PendingManual: *l.pending}
	}
}

func scanStudentCard(row pgx.Row) (domain.StudentCard, error) {
	var (
		c         domain.StudentCard
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
		return domain.StudentCard{}, err
	}
	l.apply(&c, showScore)
	return c, nil
}

// ForStudent returns the home screen's three sections.
func (s *Postgres) ForStudent(ctx context.Context, studentID string, now time.Time) (domain.StudentSections, error) {
	rows, err := s.pool.Query(ctx, studentCardColumns+studentCardFrom+`
	 WHERE a.published_at IS NOT NULL AND `+targeted+`
	 ORDER BY a.closes_at ASC, a.id DESC`, studentID)
	if err != nil {
		return domain.StudentSections{}, fmt.Errorf("assignments: list for student: %w", err)
	}
	defer rows.Close()

	var out domain.StudentSections
	for rows.Next() {
		c, err := scanStudentCard(rows)
		if err != nil {
			return domain.StudentSections{}, fmt.Errorf("assignments: scan student card: %w", err)
		}
		status := domain.Schedule.StatusAt(now, c.PublishedAt, c.OpensAt, c.ClosesAt, c.ClosedAt)
		switch {
		case c.HasLiveAttempt:
			out.DueNow = append(out.DueNow, c)
		case status == domain.Open && c.AttemptsUsed < c.MaxAttempts:
			out.DueNow = append(out.DueNow, c)
		case status == domain.Scheduled:
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
func (s *Postgres) StudentDetail(ctx context.Context, id, studentID string) (domain.StudentDetail, error) {
	var (
		d         domain.StudentDetail
		showScore bool
		l         lastAttempt
		onLimit   string
		maxPlays  *int
	)

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
		return domain.StudentDetail{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.StudentDetail{}, fmt.Errorf("assignments: student detail: %w", err)
	}
	d.Review.ShowScore = showScore
	d.Integrity.OnLimitExceeded = onLimit
	l.apply(&d.StudentCard, showScore)
	if d.HasAudio {
		d.AudioMaxPlays = maxPlays
	}
	return d, nil
}
