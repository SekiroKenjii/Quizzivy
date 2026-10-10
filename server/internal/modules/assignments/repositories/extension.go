package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/schedule"
	"quizzivy/internal/shared/visibility"

	"github.com/jackc/pgx/v5"
)

func closesOf(also string) string {
	return `
	SELECT t.title, s.id::text, ` + schedule.CloseOf("o") + `
	  FROM app.assignments a
	  JOIN app.tests t ON t.id = a.test_id
	  JOIN app.users s ON s.id IN ` + visibility.Roster("a.id") + `
	  ` + schedule.OverrideJoin("s.id") + `
	 WHERE a.id = $1::uuid AND a.published_at IS NOT NULL
	   AND s.disabled_at IS NULL
	   AND s.role_id IN (SELECT r.id FROM app.student_like_roles r)
	   AND ` + also + `
	 ORDER BY s.id`
}

var (
	closesMovedQuery   = closesOf(`(o.closes_at IS NULL OR o.closes_at < ` + schedule.CloseOf("") + `)`)
	closesGrantedQuery = closesOf(`s.id = ANY($2::uuid[]) AND o.closes_at > ` + schedule.CloseOf(""))
)

// ClosesMoved reads the enabled students of a published assignment whose own
// close the assignment's last extension changed, each with the close they now
// have. It reads the state as it stands, outside the extension's transaction.
func (s *Postgres) ClosesMoved(ctx context.Context, assignmentID string) (domain.Extension, error) {
	return s.closes(ctx, closesMovedQuery, assignmentID)
}

// ClosesGranted reads those of studentIDs, enabled students of a published
// assignment, whose override closes after the assignment's own close, each
// with the close they now have.
func (s *Postgres) ClosesGranted(ctx context.Context, assignmentID string, studentIDs []string) (domain.Extension, error) {
	return s.closes(ctx, closesGrantedQuery, assignmentID, studentIDs)
}

type closeRow struct {
	title   string
	student domain.StudentClose
}

func (s *Postgres) closes(ctx context.Context, sql string, args ...any) (domain.Extension, error) {
	rows, err := db.QueryMany(ctx, s, sql, args, func(rows pgx.Rows) (closeRow, error) {
		var r closeRow
		err := rows.Scan(&r.title, &r.student.StudentID, &r.student.ClosesAt)
		return r, err
	})
	if err != nil {
		return domain.Extension{}, fmt.Errorf("assignments: read moved closes: %w", err)
	}
	var out domain.Extension
	for _, r := range rows {
		out.Title = r.title
		out.Students = append(out.Students, r.student)
	}
	return out, nil
}
