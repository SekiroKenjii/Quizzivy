package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/paging"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// DB is a pool in production and a REPEATABLE READ transaction in tests, whose
// two readings of a global aggregate must see the same world.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Postgres struct{ db DB }

func NewPostgres(db DB) *Postgres { return &Postgres{db: db} }

var _ domain.Repository = (*Postgres)(nil)

const recentColumns = `
		SELECT at.id::text, at.student_id::text, u.full_name,
		       at.assignment_id::text, t.title, at.status::text, at.submitted_at,
		       (SELECT count(*) FROM app.attempt_answers ans
		         WHERE ans.attempt_id = at.id
		           AND ans.requires_manual AND ans.manual_score IS NULL),
		       at.flagged
		  FROM app.attempts at
		  JOIN app.users u ON u.id = at.student_id
		  JOIN app.assignments a ON a.id = at.assignment_id
		  JOIN app.tests t ON t.id = a.test_id`

func (p *Postgres) Summary(ctx context.Context) (domain.Summary, error) {
	var out domain.Summary
	err := p.db.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM app.assignments a
		    WHERE a.published_at IS NOT NULL
		      AND a.closed_at IS NULL
		      AND now() >= a.opens_at AND now() < a.closes_at),
		  (SELECT count(*)
		     FROM app.attempt_answers ans
		     JOIN app.attempts at ON at.id = ans.attempt_id
		    WHERE ans.requires_manual AND ans.manual_score IS NULL
		      AND at.status IN ('submitted', 'timed_out')),
		  (SELECT count(DISTINCT at.student_id) FROM app.attempts at
		    WHERE at.started_at >= now() - $1::interval),
		  (SELECT count(*) FROM app.attempts at WHERE at.flagged)
	`, domain.ActiveWindow).Scan(
		&out.OpenAssignments, &out.AwaitingGrading, &out.ActiveStudents, &out.FlaggedAttempts)
	if err != nil {
		return domain.Summary{}, fmt.Errorf("dashboard: counts: %w", err)
	}

	rows, err := p.db.Query(ctx, recentColumns+`
		 ORDER BY at.started_at DESC
		 LIMIT 10`)
	if err != nil {
		return domain.Summary{}, fmt.Errorf("dashboard: recent: %w", err)
	}
	out.Recent, err = scanRecent(rows, 10)
	if err != nil {
		return domain.Summary{}, err
	}
	return out, nil
}

func (p *Postgres) List(ctx context.Context, q domain.ListQuery) ([]domain.Recent, paging.Page, error) {
	number, limit, offset := paging.Clamp(q.Page, q.Limit, defaultLimit, maxLimit)

	var args []any
	where := []string{"TRUE"}
	if q.Status != nil {
		args = append(args, *q.Status)
		where = append(where, fmt.Sprintf("at.status = $%d::app.attempt_status", len(args)))
	}
	if q.Flagged != nil {
		args = append(args, *q.Flagged)
		where = append(where, fmt.Sprintf("at.flagged = $%d", len(args)))
	}
	if q.PendingGrading != nil {
		args = append(args, *q.PendingGrading)
		where = append(where, fmt.Sprintf(`(at.status IN ('submitted', 'timed_out') AND EXISTS (
		     SELECT 1 FROM app.attempt_answers ans
		      WHERE ans.attempt_id = at.id AND ans.requires_manual AND ans.manual_score IS NULL)) = $%d`, len(args)))
	}
	filter := strings.Join(where, "\n		   AND ")

	page := paging.Page{Number: number, Size: limit}
	if err := p.db.QueryRow(ctx, `SELECT count(*) FROM app.attempts at WHERE `+filter, args...).
		Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("dashboard: count attempts: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := p.db.Query(ctx, recentColumns+`
		 WHERE `+filter+fmt.Sprintf(`
		 ORDER BY at.submitted_at DESC NULLS LAST, at.started_at DESC, at.id DESC
		 LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("dashboard: list attempts: %w", err)
	}
	out, err := scanRecent(rows, limit)
	if err != nil {
		return nil, paging.Page{}, err
	}
	return out, page, nil
}

func scanRecent(rows pgx.Rows, capacity int) ([]domain.Recent, error) {
	defer rows.Close()
	out := make([]domain.Recent, 0, capacity)
	for rows.Next() {
		var r domain.Recent
		if err := rows.Scan(&r.ID, &r.StudentID, &r.StudentName, &r.AssignmentID,
			&r.TestTitle, &r.Status, &r.SubmittedAt, &r.PendingManual, &r.Flagged); err != nil {
			return nil, fmt.Errorf("dashboard: scan attempt: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
