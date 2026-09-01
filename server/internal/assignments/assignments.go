// Package assignments reads §7's assignment rows. Status is derived, never
// stored (D-18).
package assignments

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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

type Assignment struct {
	ID             string
	TestID         string
	TestVersionID  string
	TestVersion    int
	TestTitle      string
	ClassIDs       []string
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

var ErrBadCursor = fmt.Errorf("assignments: malformed cursor")

const DefaultLimit = 20
const MaxLimit = 100

type ListInput struct {
	Status *Status
	Cursor string
	Limit  int
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func encodeCursor(id string) string { return base64.RawURLEncoding.EncodeToString([]byte(id)) }

func decodeCursor(s string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", ErrBadCursor
	}
	if _, err := uuid.Parse(string(raw)); err != nil {
		return "", ErrBadCursor
	}
	return string(raw), nil
}

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
		       coalesce((SELECT array_agg(ac.class_id::text) FROM app.assignment_classes ac
		                  WHERE ac.assignment_id = a.id), '{}'),
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
		&a.ClassIDs, &a.StudentIDs,
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

// List returns one page of assignments, newest first.
//
// The status filter is applied in SQL over the same expression StatusAt
// computes, so the list and the row never disagree about what "open" means.
func (s *Store) List(ctx context.Context, in ListInput) ([]Assignment, string, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	args := []any{limit + 1}
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
	if in.Cursor != "" {
		id, err := decodeCursor(in.Cursor)
		if err != nil {
			return nil, "", err
		}
		args = append(args, id)
		where = append(where, fmt.Sprintf(`a.id < $%d::uuid`, len(args)))
	}

	sql := selectAssignment + `
		 WHERE ` + join(where) + `
		 ORDER BY a.id DESC
		 LIMIT $1`

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, "", fmt.Errorf("assignments: list: %w", err)
	}
	defer rows.Close()

	var out []Assignment
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, "", fmt.Errorf("assignments: scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("assignments: list: %w", err)
	}

	next := ""
	if len(out) > limit {
		out = out[:limit]
		next = encodeCursor(out[len(out)-1].ID)
	}
	return out, next, nil
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
