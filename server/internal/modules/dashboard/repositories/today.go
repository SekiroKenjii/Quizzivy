package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/visibility"
)

func remainingTargets() string {
	return `(SELECT count(*) FROM (
 SELECT m.user_id FROM app.assignment_classes ac
 JOIN app.class_members m ON m.class_id=ac.class_id
 WHERE ac.assignment_id=a.id AND ($1::boolean OR ac.class_id IN ` + visibility.TaughtClassIDs(2) + `)
 UNION SELECT ast.user_id FROM app.assignment_students ast
 WHERE ast.assignment_id=a.id AND ($1::boolean OR ast.user_id IN ` + visibility.StudentIDs(2) + `)
 ) roster JOIN app.users u ON u.id=roster.user_id AND u.disabled_at IS NULL
 WHERE NOT EXISTS (SELECT 1 FROM app.attempts at
 WHERE at.assignment_id=a.id AND at.student_id=roster.user_id
 AND at.status IN ('submitted','timed_out','graded')))`
}

func todaySQL() string {
	return `WITH bounds AS (SELECT ($3::timestamptz AT TIME ZONE $4)::date AS today),
 events AS (
 SELECT a.id,a.test_id,'opens' AS kind,a.opens_at AS at
 FROM app.assignments a CROSS JOIN bounds b WHERE a.published_at IS NOT NULL AND ` + ownAssignments() + `
 AND a.opens_at >= (b.today::timestamp AT TIME ZONE $4) AND a.opens_at < ((b.today+1)::timestamp AT TIME ZONE $4)
 UNION ALL
 SELECT a.id,a.test_id,'closes' AS kind,least(a.closes_at,coalesce(a.closed_at,a.closes_at)) AS at
 FROM app.assignments a CROSS JOIN bounds b WHERE a.published_at IS NOT NULL AND ` + ownAssignments() + `
 AND least(a.closes_at,coalesce(a.closed_at,a.closes_at)) >= (b.today::timestamp AT TIME ZONE $4)
 AND least(a.closes_at,coalesce(a.closed_at,a.closes_at)) < ((b.today+1)::timestamp AT TIME ZONE $4)
 )
 SELECT e.kind,e.at,e.id::text,t.title,` + remainingTargets() + `
 FROM events e JOIN app.assignments a ON a.id=e.id JOIN app.tests t ON t.id=e.test_id
 ORDER BY e.at ASC,e.id,e.kind LIMIT 20`
}

func (p *Postgres) today(ctx context.Context, q domain.HomeQuery) ([]domain.Today, error) {
	rows, err := p.Query(ctx, todaySQL(), append(homeArgs(q.Scope), q.Now, q.Zone)...)
	if err != nil {
		return nil, fmt.Errorf("dashboard: today: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Today, 0, 20)
	for rows.Next() {
		var event domain.Today
		if err := rows.Scan(&event.Kind, &event.At, &event.AssignmentID, &event.Title, &event.NotSubmitted); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}
