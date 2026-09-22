package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/paging"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

type Postgres struct{ db.Repository }

func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

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
	err := p.QueryRow(ctx, `
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
             JOIN app.users u ON u.id = at.student_id AND u.disabled_at IS NULL
		    WHERE at.started_at >= now() - $1::interval),
		  (SELECT count(*) FROM app.attempts at WHERE at.flagged),
          (SELECT count(*) FROM app.assignments a WHERE a.published_at IS NOT NULL
             AND a.closed_at IS NULL AND a.opens_at <= now() AND a.closes_at > now()
             AND a.closes_at <= now() + interval '24 hours'),
          (SELECT count(DISTINCT at.student_id) FROM app.attempts at
             JOIN app.attempt_answers ans ON ans.attempt_id = at.id
             WHERE at.status IN ('submitted','timed_out') AND ans.requires_manual AND ans.manual_score IS NULL),
          (SELECT min(at.submitted_at) FROM app.attempts at
             WHERE at.status IN ('submitted','timed_out') AND EXISTS (
               SELECT 1 FROM app.attempt_answers ans WHERE ans.attempt_id = at.id
                 AND ans.requires_manual AND ans.manual_score IS NULL)),
          (SELECT count(*) FROM app.users WHERE role = 'student' AND disabled_at IS NULL)
	`, domain.ActiveWindow).Scan(
		&out.OpenAssignments, &out.AwaitingGrading, &out.ActiveStudents, &out.FlaggedAttempts,
		&out.ClosingSoon, &out.WaitingStudents, &out.OldestWaitingAt, &out.TotalStudents)
	if err != nil {
		return domain.Summary{}, fmt.Errorf("dashboard: counts: %w", err)
	}

	out.NextClosing, err = p.nextClosing(ctx)
	if err != nil {
		return domain.Summary{}, err
	}
	rows, err := p.Query(ctx, recentColumns+`
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
	if err := p.QueryRow(ctx, `SELECT count(*) FROM app.attempts at WHERE `+filter, args...).
		Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("dashboard: count attempts: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := p.Query(ctx, recentColumns+`
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

func (p *Postgres) nextClosing(ctx context.Context) (*domain.ClosingAssignment, error) {
	var out domain.ClosingAssignment
	err := p.QueryRow(ctx, `
   SELECT a.id::text, t.title, a.closes_at,
          (SELECT count(DISTINCT at.student_id) FROM app.attempts at
             JOIN app.users u ON u.id = at.student_id AND u.disabled_at IS NULL
            WHERE at.assignment_id = a.id AND at.status IN ('submitted','timed_out','graded')),
          (SELECT count(*) FROM (
             SELECT m.user_id FROM app.assignment_classes ac
               JOIN app.class_members m ON m.class_id = ac.class_id WHERE ac.assignment_id = a.id
             UNION SELECT ast.user_id FROM app.assignment_students ast WHERE ast.assignment_id = a.id
           ) roster JOIN app.users u ON u.id = roster.user_id AND u.disabled_at IS NULL)
     FROM app.assignments a JOIN app.tests t ON t.id = a.test_id
    WHERE a.published_at IS NOT NULL AND a.closed_at IS NULL
      AND a.opens_at <= now() AND a.closes_at > now() AND a.closes_at <= now() + interval '24 hours'
    ORDER BY a.closes_at, a.id LIMIT 1
 `).Scan(&out.ID, &out.Title, &out.ClosesAt, &out.SubmittedCount, &out.TargetCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dashboard: nearest closing assignment: %w", err)
	}
	return &out, nil
}
