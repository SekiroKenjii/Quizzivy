package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/dashboard/domain"
)

func submissionsSQL() string {
	return `WITH bounds AS (
 SELECT ($3::timestamptz AT TIME ZONE $4)::date AS today
 ), papers AS (
 SELECT (at.submitted_at AT TIME ZONE $4)::date AS day,at.status,
        at.score_earned / at.score_total AS ratio
 FROM app.attempts at CROSS JOIN bounds b
 WHERE ` + ownPapers() + ` AND at.status IN ('submitted','timed_out','graded')
 AND at.submitted_at >= ((b.today-$5::integer+1)::timestamp AT TIME ZONE $4)
 AND at.submitted_at < ((b.today+1)::timestamp AT TIME ZONE $4)
 ), daily AS (SELECT day,count(*) AS count FROM papers GROUP BY day),
 figures AS (SELECT count(*) AS total,round(100*avg(ratio) FILTER (WHERE status='graded'))::integer AS average FROM papers)
 SELECT (b.today-$5::integer+1+g.n)::text,coalesce(d.count,0),f.total,f.average
 FROM bounds b CROSS JOIN generate_series(0,$5::integer-1) g(n) CROSS JOIN figures f
 LEFT JOIN daily d ON d.day=b.today-$5::integer+1+g.n ORDER BY g.n`
}

func (p *Postgres) submissions(ctx context.Context, q domain.HomeQuery) (domain.Submissions, error) {
	out := domain.Submissions{Days: make([]domain.Day, 0, q.Days)}
	rows, err := p.Query(ctx, submissionsSQL(), append(homeArgs(q.Scope), q.Now, q.Zone, q.Days)...)
	if err != nil {
		return out, fmt.Errorf("dashboard: submissions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var day domain.Day
		if err := rows.Scan(&day.Date, &day.Count, &out.Total, &out.AveragePercent); err != nil {
			return out, err
		}
		out.Days = append(out.Days, day)
	}
	return out, rows.Err()
}
