// Package assignments reads §7's assignment rows. Status is derived, never
// stored (D-18).
package assignments

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/paging"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Status string

const (
	Draft     Status = "draft"
	Scheduled Status = "scheduled"
	Open      Status = "open"
	Closed    Status = "closed"
)

// StatusAt is D-18's pure function: no scheduler, no stale row.
//
// The draft case does not weaken that. Publishing is an act by the teacher, not
// a timestamp arriving, so nothing has to flip a row when a clock passes -- the
// window rule reads exactly as it did once publishedAt exists.
func StatusAt(now time.Time, publishedAt *time.Time, opensAt, closesAt time.Time, closedAt *time.Time) Status {
	if publishedAt == nil {
		return Draft
	}
	if closedAt != nil && !now.Before(*closedAt) {
		return Closed
	}
	switch {
	case now.Before(opensAt):
		return Scheduled
	case now.Before(closesAt):
		return Open
	default:
		return Closed
	}
}

type Review struct {
	ShowScore, ShowCorrectAnswers, ShowExplanations bool
}

type Integrity struct {
	RequireFullscreen bool
	BlockCopyPaste    bool
	MaxFocusLoss      int
	OnLimitExceeded   string
	MinAwayMs         int
}

// ClassRef is a targeted class and its name. The name travels with the
// assignment because every screen that lists one names its classes, and the
// alternative -- looking the name up in the classes list -- reads one page of
// it, so it answers with an em dash for any class past the page boundary.
type ClassRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Assignment struct {
	ID             string
	TestID         string
	TestVersionID  string
	TestVersion    int
	TestTitle      string
	Classes        []ClassRef
	StudentIDs     []string
	OpensAt        time.Time
	ClosesAt       time.Time
	ClosedAt       *time.Time
	PublishedAt    *time.Time
	DurationMin    int
	MaxAttempts    int
	ShuffleQ       bool
	ShuffleO       bool
	Review         Review
	Integrity      Integrity
	SubmittedCount int
	TargetCount    int
	FlaggedCount   int
}

const DefaultLimit = 20
const MaxLimit = 100

type ListInput struct {
	Status *Status
	Page   int
	Limit  int
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// selectAssignment is shared by List and Get so a row can never mean one thing
// in the list and another on the detail screen.
const selectAssignment = `
		SELECT a.id::text, a.test_id::text, a.test_version_id::text, v.version, t.title,
		       a.opens_at, a.closes_at, a.closed_at, a.published_at,
		       a.duration_minutes, a.max_attempts, a.shuffle_questions, a.shuffle_options,
		       a.review_show_score, a.review_show_correct_answers, a.review_show_explanations,
		       a.integrity_require_fullscreen, a.integrity_block_copy_paste,
		       a.integrity_max_focus_loss, a.integrity_on_limit_exceeded::text,
		       a.integrity_min_away_ms,
		       coalesce((SELECT jsonb_agg(jsonb_build_object('id', c.id::text, 'name', c.name)
		                                  ORDER BY c.name)
		                   FROM app.assignment_classes ac
		                   JOIN app.classes c ON c.id = ac.class_id
		                  WHERE ac.assignment_id = a.id), '[]'::jsonb),
		       coalesce((SELECT array_agg(ast.user_id::text) FROM app.assignment_students ast
		                  WHERE ast.assignment_id = a.id), '{}'),
		       -- Both sides of submitted/total range over the SAME set: students
		       -- who are expected to do the work. A disabled account is not, so
		       -- leaving it in the denominator pinned every assignment at
		       -- "12/13" with nothing able to close the gap.
		       (SELECT count(*) FROM app.attempts at
		          JOIN app.users u ON u.id = at.student_id AND u.disabled_at IS NULL
		         -- timed_out counts as handed in: the student is done, whatever
		         -- ended it, and 12/13 must not read 11/13 because one ran out
		         -- of time. Matches students.go's submitted_count.
		         WHERE at.assignment_id = a.id
		           AND at.status IN ('submitted','timed_out','graded')),
		       (SELECT count(*) FROM app.attempts at
		          JOIN app.users u ON u.id = at.student_id AND u.disabled_at IS NULL
		         WHERE at.assignment_id = a.id AND at.flagged),
		       -- One roster, not two counts added together: a student reached
		       -- both through their class and by name is one person, and a
		       -- total larger than the class can never read 13/13.
		       (SELECT count(*) FROM (
		            SELECT m.user_id
		              FROM app.assignment_classes ac
		              JOIN app.class_members m ON m.class_id = ac.class_id
		             WHERE ac.assignment_id = a.id
		            UNION
		            SELECT ast.user_id FROM app.assignment_students ast
		             WHERE ast.assignment_id = a.id
		        ) roster
		        JOIN app.users u ON u.id = roster.user_id AND u.disabled_at IS NULL)
		  FROM app.assignments a
		  JOIN app.tests t ON t.id = a.test_id
		  JOIN app.test_versions v ON v.id = a.test_version_id
`

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func scanAssignment(row pgx.Row) (Assignment, error) {
	var a Assignment
	err := row.Scan(&a.ID, &a.TestID, &a.TestVersionID, &a.TestVersion, &a.TestTitle,
		&a.OpensAt, &a.ClosesAt, &a.ClosedAt, &a.PublishedAt,
		&a.DurationMin, &a.MaxAttempts, &a.ShuffleQ, &a.ShuffleO,
		&a.Review.ShowScore, &a.Review.ShowCorrectAnswers, &a.Review.ShowExplanations,
		&a.Integrity.RequireFullscreen, &a.Integrity.BlockCopyPaste,
		&a.Integrity.MaxFocusLoss, &a.Integrity.OnLimitExceeded, &a.Integrity.MinAwayMs,
		&a.Classes, &a.StudentIDs,
		&a.SubmittedCount, &a.FlaggedCount, &a.TargetCount)
	return a, err
}

// Get returns one assignment.
func (s *Store) Get(ctx context.Context, id string) (Assignment, error) {
	return s.get(ctx, s.pool, id)
}

func (s *Store) get(ctx context.Context, q querier, id string) (Assignment, error) {
	a, err := scanAssignment(q.QueryRow(ctx, selectAssignment+`
		 WHERE a.id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Assignment{}, ErrNotFound
	}
	if err != nil {
		return Assignment{}, fmt.Errorf("assignments: get: %w", err)
	}
	return a, nil
}

// List returns one page of assignments, newest first, with the paging
// beside it (O-20: OFFSET, so the client can draw numbered pages).
//
// The status filter is applied in SQL over the same expression StatusAt
// computes, so the list and the row never disagree about what "open" means.
func (s *Store) List(ctx context.Context, in ListInput) ([]Assignment, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	var args []any
	where := []string{"TRUE"}
	if in.Status != nil {
		args = append(args, string(*in.Status))
		where = append(where, fmt.Sprintf(`
			CASE
			  WHEN a.published_at IS NULL THEN 'draft'
			  WHEN a.closed_at IS NOT NULL AND now() >= a.closed_at THEN 'closed'
			  WHEN now() < a.opens_at THEN 'scheduled'
			  WHEN now() < a.closes_at THEN 'open'
			  ELSE 'closed'
			END = $%d`, len(args)))
	}

	page := paging.Page{Number: number, Size: limit}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM app.assignments a WHERE `+join(where), args...).
		Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("assignments: count: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, selectAssignment+`
		 WHERE `+join(where)+fmt.Sprintf(`
		 ORDER BY a.id DESC
		 LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("assignments: list: %w", err)
	}
	defer rows.Close()

	out := make([]Assignment, 0, limit)
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, paging.Page{}, fmt.Errorf("assignments: scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, paging.Page{}, fmt.Errorf("assignments: list: %w", err)
	}
	return out, page, nil
}

func join(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "\n		   AND "
		}
		out += p
	}
	return out
}
