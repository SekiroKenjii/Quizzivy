package attempts

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// Score is what a closed attempt is worth so far.
type Score struct {
	Earned        float64
	Total         float64
	PendingManual int
}

// MonitorRow is one targeted student on G-02, whether or not they have started.
type MonitorRow struct {
	StudentID string
	FullName  string
	// State is `not_started`, or the attempt's status.
	State          string
	AttemptID      *string
	AttemptNo      *int
	StartedAt      *time.Time
	DeadlineAt     *time.Time
	SubmittedAt    *time.Time
	AnsweredCount  *int
	Score          *Score
	FocusLossCount *int
	Flagged        bool
	AudioOverLimit bool
}

// Monitor is the §8 monitor screen's data: the roster, each with the attempt
// that stands for them.
type Monitor struct {
	ServerTime    time.Time
	QuestionCount int
	Rows          []MonitorRow
}

// Monitor answers G-02 in two queries -- the roster and the attempts -- never
// one per student (§13.8). The roster is the left side: a student who has not
// started is a row, not an absence.
func (s *Store) Monitor(ctx context.Context, assignmentID string, now time.Time) (Monitor, error) {
	out := Monitor{ServerTime: now}

	rows, err := s.pool.Query(ctx, `
		WITH a AS (
		  SELECT id, test_version_id FROM app.assignments WHERE id = $1::uuid
		), n AS (
		  SELECT count(*) AS questions
		    FROM a
		    JOIN app.test_version_sections s ON s.test_version_id = a.test_version_id
		    JOIN app.test_version_questions q ON q.test_version_section_id = s.id
		), roster AS (
		  SELECT m.user_id
		    FROM a
		    JOIN app.assignment_classes ac ON ac.assignment_id = a.id
		    JOIN app.class_members m ON m.class_id = ac.class_id
		  UNION
		  SELECT ast.user_id
		    FROM a
		    JOIN app.assignment_students ast ON ast.assignment_id = a.id
		)
		SELECT u.id::text, u.full_name, n.questions
		  FROM a, n
		  LEFT JOIN roster ON true
		  LEFT JOIN app.users u ON u.id = roster.user_id AND u.disabled_at IS NULL
		 ORDER BY u.full_name, u.id`, assignmentID)
	if err != nil {
		return Monitor{}, fmt.Errorf("attempts: monitor roster: %w", err)
	}
	defer rows.Close()

	found := false
	at := map[string]int{}
	for rows.Next() {
		var id, name *string
		found = true
		if err := rows.Scan(&id, &name, &out.QuestionCount); err != nil {
			return Monitor{}, fmt.Errorf("attempts: scan roster: %w", err)
		}
		if id == nil {
			continue
		}
		at[*id] = len(out.Rows)
		out.Rows = append(out.Rows, MonitorRow{StudentID: *id, FullName: *name, State: "not_started"})
	}
	if err := rows.Err(); err != nil {
		return Monitor{}, fmt.Errorf("attempts: monitor roster: %w", err)
	}
	rows.Close()
	if !found {
		return Monitor{}, ErrNotFound
	}
	if len(out.Rows) == 0 {
		return out, nil
	}

	if err := s.attachAttempts(ctx, assignmentID, out.Rows, at); err != nil {
		return Monitor{}, err
	}
	sort.SliceStable(out.Rows, func(i, j int) bool { return before(out.Rows[i], out.Rows[j]) })
	return out, nil
}

// attachAttempts picks one attempt per student: the latest that still counts,
// or the latest voided one when nothing else exists.
func (s *Store) attachAttempts(ctx context.Context, assignmentID string, rows []MonitorRow, at map[string]int) error {
	found, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (at.student_id)
		       at.student_id::text, at.id::text, at.attempt_no, at.status::text,
		       at.started_at, at.deadline_at, at.submitted_at,
		       at.score_earned, at.score_total, at.focus_loss_count, at.flagged,
		       (SELECT count(*) FROM app.attempt_answers aa WHERE aa.attempt_id = at.id),
		       (SELECT count(*) FROM app.attempt_answers aa
		         WHERE aa.attempt_id = at.id AND aa.requires_manual AND aa.manual_score IS NULL),
		       EXISTS (
		         SELECT 1 FROM app.attempt_audio_plays p
		           JOIN app.test_version_questions q ON q.id = p.question_id
		          WHERE p.attempt_id = at.id
		            AND q.audio_max_plays IS NOT NULL AND p.plays > q.audio_max_plays)
		  FROM app.attempts at
		 WHERE at.assignment_id = $1::uuid
		 ORDER BY at.student_id, (at.status <> 'voided') DESC, at.attempt_no DESC`, assignmentID)
	if err != nil {
		return fmt.Errorf("attempts: monitor attempts: %w", err)
	}
	defer found.Close()

	for found.Next() {
		var (
			studentID, id, status   string
			no, focusLoss, answered int
			pending                 int
			started, deadline       time.Time
			submitted               *time.Time
			earned, total           *float64
			flagged, overLimit      bool
		)
		if err := found.Scan(&studentID, &id, &no, &status, &started, &deadline, &submitted,
			&earned, &total, &focusLoss, &flagged, &answered, &pending, &overLimit); err != nil {
			return fmt.Errorf("attempts: scan monitor attempt: %w", err)
		}
		i, ok := at[studentID]
		if !ok {
			continue
		}
		row := &rows[i]
		row.State = status
		row.AttemptID = &id
		row.AttemptNo = &no
		row.StartedAt = &started
		row.DeadlineAt = &deadline
		row.SubmittedAt = submitted
		row.AnsweredCount = &answered
		row.FocusLossCount = &focusLoss
		row.Flagged = flagged
		row.AudioOverLimit = overLimit
		if earned != nil && total != nil {
			row.Score = &Score{Earned: *earned, Total: *total, PendingManual: pending}
		}
	}
	return found.Err()
}

// before is G-02's order: the rows that need a decision float up -- in
// progress with the least time left, then not started, then everything
// settled, by name.
func before(a, b MonitorRow) bool {
	ra, rb := stateRank(a.State), stateRank(b.State)
	if ra != rb {
		return ra < rb
	}
	if a.State == string(InProgress) && a.DeadlineAt != nil && b.DeadlineAt != nil &&
		!a.DeadlineAt.Equal(*b.DeadlineAt) {
		return a.DeadlineAt.Before(*b.DeadlineAt)
	}
	if a.FullName != b.FullName {
		return a.FullName < b.FullName
	}
	return a.StudentID < b.StudentID
}

func stateRank(state string) int {
	switch Status(state) {
	case InProgress:
		return 0
	case "not_started":
		return 1
	case TimedOut:
		return 2
	case Submitted:
		return 3
	case Graded:
		return 4
	default:
		return 5
	}
}

func (s *Service) Monitor(ctx context.Context, assignmentID string) (Monitor, error) {
	if err := s.expireDue(ctx, assignmentID); err != nil {
		return Monitor{}, err
	}
	return s.store.Monitor(ctx, assignmentID, s.now())
}

// expireDue closes every attempt on the assignment whose time has run out, so
// the monitor never shows "in progress" beside a deadline in the past.
func (s *Service) expireDue(ctx context.Context, assignmentID string) error {
	ids, err := s.store.dueAttempts(ctx, assignmentID, s.now())
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.store.ExpireIfDue(ctx, id, s.now()); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) dueAttempts(ctx context.Context, assignmentID string, now time.Time) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text FROM app.attempts
		 WHERE assignment_id = $1::uuid AND status = 'in_progress' AND deadline_at < $2`,
		assignmentID, now)
	if err != nil {
		return nil, fmt.Errorf("attempts: find due attempts: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("attempts: scan due attempt: %w", err)
		}
		ids = append(ids, id)
	}
	if errors.Is(rows.Err(), pgx.ErrNoRows) {
		return nil, nil
	}
	return ids, rows.Err()
}
