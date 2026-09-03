// Package dashboard answers §8's /admin in one round trip.
package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Recent is one row of the "what just happened" list.
type Recent struct {
	ID            string
	StudentID     string
	StudentName   string
	AssignmentID  string
	TestTitle     string
	Status        string
	SubmittedAt   *time.Time
	PendingManual int
	Flagged       bool
}

// Summary is the five figures §8 asks for, and spec §15 has no endpoint for.
type Summary struct {
	OpenAssignments int
	AwaitingGrading int
	ActiveStudents  int
	FlaggedAttempts int
	Recent          []Recent
}

// DB is what the store reads through: a pool in production, and in tests a
// transaction -- every figure here is a global aggregate, and a test that
// takes two readings of one can only trust the difference if nothing else
// on the shared database can move it in between. A REPEATABLE READ
// transaction is exactly that isolation, and it needs no scope parameter
// smuggled into production queries to get it.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Store struct{ db DB }

func NewStore(db DB) *Store { return &Store{db: db} }

// ActiveWindow is what "active" means for the student count: §8's dashboard is
// a work queue for today, so a student who sat something last month is not part
// of the number the teacher is reading.
const ActiveWindow = 7 * 24 * time.Hour

// Get returns the summary.
//
// One statement rather than five round trips, which is what the contract
// promises. Each count is a scalar subquery over its own partial index --
// attempts_grading_queue_idx and attempts_flagged_idx exist for exactly these
// two and are near-empty most of the time.
func (s *Store) Get(ctx context.Context) (Summary, error) {
	var out Summary
	err := s.db.QueryRow(ctx, `
		SELECT
		  -- published_at NOT NULL: a draft whose window happens to be current
		  -- is not open, because nobody has been given it.
		  (SELECT count(*) FROM app.assignments a
		    WHERE a.published_at IS NOT NULL
		      AND a.closed_at IS NULL
		      AND now() >= a.opens_at AND now() < a.closes_at),
		  (SELECT count(*)
		     FROM app.attempt_answers ans
		     JOIN app.attempts at ON at.id = ans.attempt_id
		    WHERE ans.requires_manual AND ans.manual_score IS NULL
		      -- Both closed-but-ungraded states. An attempt that ran out of
		      -- time still has essays in it (00025).
		      AND at.status IN ('submitted', 'timed_out')),
		  (SELECT count(DISTINCT at.student_id) FROM app.attempts at
		    WHERE at.started_at >= now() - $1::interval),
		  (SELECT count(*) FROM app.attempts at WHERE at.flagged)
	`, ActiveWindow).Scan(
		&out.OpenAssignments, &out.AwaitingGrading, &out.ActiveStudents, &out.FlaggedAttempts)
	if err != nil {
		return Summary{}, fmt.Errorf("dashboard: counts: %w", err)
	}

	out.Recent, err = s.recent(ctx)
	if err != nil {
		return Summary{}, err
	}
	return out, nil
}

func (s *Store) recent(ctx context.Context) ([]Recent, error) {
	rows, err := s.db.Query(ctx, `
		SELECT at.id::text, at.student_id::text, u.full_name,
		       at.assignment_id::text, t.title, at.status::text, at.submitted_at,
		       (SELECT count(*) FROM app.attempt_answers ans
		         WHERE ans.attempt_id = at.id
		           AND ans.requires_manual AND ans.manual_score IS NULL),
		       at.flagged
		  FROM app.attempts at
		  JOIN app.users u ON u.id = at.student_id
		  JOIN app.assignments a ON a.id = at.assignment_id
		  JOIN app.tests t ON t.id = a.test_id
		 ORDER BY at.started_at DESC
		 LIMIT 10`)
	if err != nil {
		return nil, fmt.Errorf("dashboard: recent: %w", err)
	}
	defer rows.Close()

	var out []Recent
	for rows.Next() {
		var r Recent
		if err := rows.Scan(&r.ID, &r.StudentID, &r.StudentName, &r.AssignmentID,
			&r.TestTitle, &r.Status, &r.SubmittedAt, &r.PendingManual, &r.Flagged); err != nil {
			return nil, fmt.Errorf("dashboard: scan recent: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
