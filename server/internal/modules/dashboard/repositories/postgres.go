package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/visibility"
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

const (
	attemptAssignment = "at.assignment_id"
	attemptStudent    = "at.student_id"
)

const studentLike = ` AND u.role_id IN (SELECT r.id FROM app.student_like_roles r)`

type reach struct {
	args        []any
	assignments string
	papers      string
	active      string
	students    string
}

func reachOf(scope access.Scope) reach {
	if scope.All {
		return reach{students: `(SELECT count(*) FROM app.users u WHERE u.disabled_at IS NULL` + studentLike + `)`}
	}
	members := studentLike + ` AND u.id IN ` + visibility.TaughtMemberIDs(2)
	papers := ` AND at.assignment_id IN ` + visibility.AssignmentIDs(2) + ` AND ` + visibility.Papers(1, 2, attemptAssignment, attemptStudent)
	return reach{
		args:        []any{false, opt.String(scope.UserID)},
		assignments: ` AND a.id IN ` + visibility.AssignmentIDs(2),
		papers:      papers,
		active:      ` AND at.assignment_id IN ` + visibility.AssignmentIDs(2) + members,
		students:    `(SELECT count(*) FROM app.users u WHERE u.disabled_at IS NULL` + members + `)`,
	}
}

// Summary reads the teacher's home over what the scope reaches: the
// assignments visibility.AssignmentIDs gives, the papers visibility.Papers
// shows on them, and the student-like members of the classes the scope
// teaches. Under scope.all it reads every row, exactly as before scoping; a
// zero scope reads nothing.
func (p *Postgres) Summary(ctx context.Context, scope access.Scope) (domain.Summary, error) {
	r := reachOf(scope)
	window := fmt.Sprintf(`$%d`, len(r.args)+1)
	var out domain.Summary
	err := p.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM app.assignments a
		    WHERE a.published_at IS NOT NULL
		      AND a.closed_at IS NULL
		      AND now() >= a.opens_at AND now() < a.closes_at`+r.assignments+`),
		  (SELECT count(*)
		     FROM app.attempt_answers ans
		     JOIN app.attempts at ON at.id = ans.attempt_id
		    WHERE ans.requires_manual AND ans.manual_score IS NULL
		      AND at.status IN ('submitted', 'timed_out')`+r.papers+`),
		  (SELECT count(DISTINCT at.student_id) FROM app.attempts at
             JOIN app.users u ON u.id = at.student_id AND u.disabled_at IS NULL
		    WHERE at.started_at >= now() - `+window+`::interval`+r.active+`),
		  (SELECT count(*) FROM app.attempts at WHERE at.flagged`+r.papers+`),
          (SELECT count(*) FROM app.assignments a WHERE a.published_at IS NOT NULL
             AND a.closed_at IS NULL AND a.opens_at <= now() AND a.closes_at > now()
             AND a.closes_at <= now() + interval '24 hours'`+r.assignments+`),
          (SELECT count(DISTINCT at.student_id) FROM app.attempts at
             JOIN app.attempt_answers ans ON ans.attempt_id = at.id
             WHERE at.status IN ('submitted','timed_out') AND ans.requires_manual AND ans.manual_score IS NULL`+r.papers+`),
          (SELECT min(at.submitted_at) FROM app.attempts at
             WHERE at.status IN ('submitted','timed_out') AND EXISTS (
               SELECT 1 FROM app.attempt_answers ans WHERE ans.attempt_id = at.id
                 AND ans.requires_manual AND ans.manual_score IS NULL)`+r.papers+`),
          `+r.students+`
	`, append(r.args, domain.ActiveWindow)...).Scan(
		&out.OpenAssignments, &out.AwaitingGrading, &out.ActiveStudents, &out.FlaggedAttempts,
		&out.ClosingSoon, &out.WaitingStudents, &out.OldestWaitingAt, &out.TotalStudents)
	if err != nil {
		return domain.Summary{}, fmt.Errorf("dashboard: counts: %w", err)
	}

	out.NextClosing, err = p.nextClosing(ctx, scope)
	if err != nil {
		return domain.Summary{}, err
	}
	recent := ``
	if !scope.All {
		recent = `
		 WHERE TRUE` + r.papers
	}
	rows, err := p.Query(ctx, recentColumns+recent+`
		 ORDER BY at.started_at DESC
		 LIMIT 10`, r.args...)
	if err != nil {
		return domain.Summary{}, fmt.Errorf("dashboard: recent: %w", err)
	}
	out.Recent, err = scanRecent(rows, 10)
	if err != nil {
		return domain.Summary{}, err
	}
	return out, nil
}

// List returns one page of the attempts on assignments the query's scope
// reaches, as visibility.Papers shows them, newest hand-in first.
func (p *Postgres) List(ctx context.Context, q domain.ListQuery) ([]domain.Recent, paging.Page, error) {
	number, limit, offset := paging.Clamp(q.Page, q.Limit, defaultLimit, maxLimit)

	var args []any
	where := []string{"TRUE"}
	if !q.Scope.All {
		args = append(args, q.Scope.All, opt.String(q.Scope.UserID))
		where = append(where, `at.assignment_id IN `+visibility.AssignmentIDs(2),
			visibility.Papers(1, 2, "at.assignment_id", "at.student_id"))
	}
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

func (p *Postgres) nextClosing(ctx context.Context, scope access.Scope) (*domain.ClosingAssignment, error) {
	var args []any
	var classes, students, reached string
	if !scope.All {
		args = []any{opt.String(scope.UserID)}
		classes = ` AND ac.class_id IN ` + visibility.TaughtClassIDs(1)
		students = ` AND ast.user_id IN ` + visibility.StudentIDs(1)
		reached = ` AND a.id IN ` + visibility.AssignmentIDs(1)
	}
	var out domain.ClosingAssignment
	err := p.QueryRow(ctx, `
   SELECT a.id::text, t.title, a.closes_at,
          (SELECT count(DISTINCT at.student_id) FROM app.attempts at
             JOIN app.users u ON u.id = at.student_id AND u.disabled_at IS NULL
            WHERE at.assignment_id = a.id AND at.status IN ('submitted','timed_out','graded')),
          (SELECT count(*) FROM (
             SELECT m.user_id FROM app.assignment_classes ac
               JOIN app.class_members m ON m.class_id = ac.class_id WHERE ac.assignment_id = a.id`+classes+`
             UNION SELECT ast.user_id FROM app.assignment_students ast WHERE ast.assignment_id = a.id`+students+`
           ) roster JOIN app.users u ON u.id = roster.user_id AND u.disabled_at IS NULL)
     FROM app.assignments a JOIN app.tests t ON t.id = a.test_id
    WHERE a.published_at IS NOT NULL AND a.closed_at IS NULL
      AND a.opens_at <= now() AND a.closes_at > now() AND a.closes_at <= now() + interval '24 hours'`+reached+`
    ORDER BY a.closes_at, a.id LIMIT 1
 `, args...).Scan(&out.ID, &out.Title, &out.ClosesAt, &out.SubmittedCount, &out.TargetCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dashboard: nearest closing assignment: %w", err)
	}
	return &out, nil
}
