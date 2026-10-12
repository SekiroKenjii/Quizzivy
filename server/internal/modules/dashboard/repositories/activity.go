package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/visibility"
)

func activitySQL() string {
	return `WITH activity AS (
 SELECT at.id,0 AS source,
 CASE WHEN at.status='in_progress' THEN 'started' ELSE 'submitted' END AS kind,
 CASE WHEN at.status='in_progress' THEN at.started_at ELSE at.submitted_at END AS at,
 u.full_name AS student_name,t.title AS subject,at.flagged
 FROM app.attempts at JOIN app.users u ON u.id=at.student_id
 JOIN app.assignments a ON a.id=at.assignment_id JOIN app.tests t ON t.id=a.test_id
 WHERE ` + ownPapers() + ` AND at.status IN ('in_progress','submitted','timed_out','graded')
 UNION ALL
 SELECT m.user_id,1 AS source,'joined',m.joined_at,u.full_name,c.name,false
 FROM app.class_members m JOIN app.classes c ON c.id=m.class_id JOIN app.users u ON u.id=m.user_id
 WHERE ($1::boolean OR c.id IN ` + visibility.TaughtClassIDs(2) + `)
 AND m.joined_via <> 'admin' AND u.role_id IN (SELECT r.id FROM app.student_like_roles r)
 ) SELECT kind,at,student_name,subject,flagged FROM activity WHERE at IS NOT NULL
 ORDER BY at DESC,source,id,subject LIMIT 10`
}

func (p *Postgres) activity(ctx context.Context, scope access.Scope) ([]domain.Activity, error) {
	rows, err := p.Query(ctx, activitySQL(), homeArgs(scope)...)
	if err != nil {
		return nil, fmt.Errorf("dashboard: activity: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Activity, 0, 10)
	for rows.Next() {
		var event domain.Activity
		if err := rows.Scan(&event.Kind, &event.At, &event.StudentName, &event.Subject, &event.Flagged); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}
