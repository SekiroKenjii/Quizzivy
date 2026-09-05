package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/paging"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const DefaultLimit = 20

const MaxLimit = 100

// DB is what the store queries through: the pool in production, and a
// transaction in a test that needs one consistent snapshot of tables every
// package on the shared database inserts into.
type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Postgres struct{ pool DB }

func NewPostgres(db DB) *Postgres { return &Postgres{pool: db} }

const selectAssignment = `
		SELECT a.id::text, a.test_id::text, a.test_version_id::text, v.version, t.title,
		       a.opens_at, a.closes_at, a.closed_at, a.published_at,
		       a.duration_minutes, a.max_attempts, a.shuffle_questions, a.shuffle_options,
		       a.review_show_score, a.review_show_correct_answers, a.review_show_explanations,
		       a.integrity_require_fullscreen, a.integrity_block_copy_paste,
		       a.integrity_max_focus_loss, a.integrity_on_limit_exceeded::text,
		       a.integrity_min_away_ms,
		       coalesce((SELECT jsonb_agg(jsonb_build_object('id', c.id::text, 'name', c.name,
		                                  'studentCount', (SELECT count(*) FROM app.class_members m
		                                                     JOIN app.users u ON u.id = m.user_id AND u.disabled_at IS NULL
		                                                    WHERE m.class_id = c.id))
		                                  ORDER BY c.name)
		                   FROM app.assignment_classes ac
		                   JOIN app.classes c ON c.id = ac.class_id
		                  WHERE ac.assignment_id = a.id), '[]'::jsonb),
		       coalesce((SELECT jsonb_agg(jsonb_build_object('id', u.id::text, 'name', u.full_name)
		                                  ORDER BY u.full_name)
		                   FROM app.assignment_students ast
		                   JOIN app.users u ON u.id = ast.user_id
		                  WHERE ast.assignment_id = a.id), '[]'::jsonb),
		       a.updated_at,
		       (SELECT count(DISTINCT aa.attempt_id) FROM app.attempt_answers aa
		          JOIN app.attempts at ON at.id = aa.attempt_id
		         WHERE at.assignment_id = a.id
		           AND at.status IN ('submitted','timed_out')
		           AND aa.requires_manual AND aa.manual_score IS NULL),
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

func scanAssignment(row pgx.Row) (domain.Assignment, error) {
	var a domain.Assignment
	err := row.Scan(&a.ID, &a.TestID, &a.TestVersionID, &a.TestVersion, &a.TestTitle,
		&a.OpensAt, &a.ClosesAt, &a.ClosedAt, &a.PublishedAt,
		&a.DurationMin, &a.MaxAttempts, &a.ShuffleQ, &a.ShuffleO,
		&a.Review.ShowScore, &a.Review.ShowCorrectAnswers, &a.Review.ShowExplanations,
		&a.Integrity.RequireFullscreen, &a.Integrity.BlockCopyPaste,
		&a.Integrity.MaxFocusLoss, &a.Integrity.OnLimitExceeded, &a.Integrity.MinAwayMs,
		&a.Classes, &a.Students, &a.UpdatedAt, &a.PendingGradingCount,
		&a.SubmittedCount, &a.FlaggedCount, &a.TargetCount)
	return a, err
}

// Get returns one assignment.
func (s *Postgres) Get(ctx context.Context, id string) (domain.Assignment, error) {
	return s.get(ctx, s.pool, id)
}

func (s *Postgres) get(ctx context.Context, q querier, id string) (domain.Assignment, error) {
	a, err := scanAssignment(q.QueryRow(ctx, selectAssignment+`
		 WHERE a.id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Assignment{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: get: %w", err)
	}
	return a, nil
}

const derivedStatus = `
			CASE
			  WHEN a.published_at IS NULL THEN 'draft'
			  WHEN a.closed_at IS NOT NULL AND now() >= a.closed_at THEN 'closed'
			  WHEN now() < a.opens_at THEN 'scheduled'
			  WHEN now() < a.closes_at THEN 'open'
			  ELSE 'closed'
			END`

// Facets counts every status within the same narrowing List applies, minus
// the status itself, so the tabs never disagree with the rows.
func (s *Postgres) Facets(ctx context.Context, in domain.ListInput) (domain.Facets, error) {
	where, args := narrow(domain.ListInput{ClassID: in.ClassID})
	var f domain.Facets
	err := s.pool.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE `+derivedStatus+` = 'draft'),
		       count(*) FILTER (WHERE `+derivedStatus+` = 'scheduled'),
		       count(*) FILTER (WHERE `+derivedStatus+` = 'open'),
		       count(*) FILTER (WHERE `+derivedStatus+` = 'closed')
		  FROM app.assignments a
		 WHERE `+join(where), args...).Scan(&f.All, &f.Draft, &f.Scheduled, &f.Open, &f.Closed)
	if err != nil {
		return domain.Facets{}, fmt.Errorf("assignments: facets: %w", err)
	}
	return f, nil
}

func narrow(in domain.ListInput) ([]string, []any) {
	var args []any
	where := []string{"TRUE"}
	if in.Status != nil {
		args = append(args, string(*in.Status))
		where = append(where, fmt.Sprintf(derivedStatus+` = $%d`, len(args)))
	}
	if in.ClassID != nil {
		args = append(args, *in.ClassID)
		where = append(where, fmt.Sprintf(`EXISTS (SELECT 1 FROM app.assignment_classes ac
		                   WHERE ac.assignment_id = a.id AND ac.class_id = $%d::uuid)`, len(args)))
	}
	return where, args
}

func (s *Postgres) List(ctx context.Context, in domain.ListInput) ([]domain.Assignment, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)
	where, args := narrow(in)

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

	out := make([]domain.Assignment, 0, limit)
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
