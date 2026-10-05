package repositories

import (
	"context"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/visibility"
)

func homeArgs(scope access.Scope) []any { return []any{scope.All, opt.String(scope.UserID)} }

func ownAssignments() string { return `($1::boolean OR a.id IN ` + visibility.AssignmentIDs(2) + `)` }

func ownPapers() string {
	return `($1::boolean OR at.assignment_id IN ` + visibility.AssignmentIDs(2) + `) AND ` + visibility.Papers(1, 2, "at.assignment_id", "at.student_id")
}

// Home reads the new dashboard figures at the supplied clock and calendar zone.
func (p *Postgres) Home(ctx context.Context, q domain.HomeQuery) (domain.Home, error) {
	var out domain.Home
	var err error
	out.TakingNow, err = p.takingNow(ctx, q)
	if err != nil {
		return out, err
	}
	out.Submissions, err = p.submissions(ctx, q)
	if err != nil {
		return out, err
	}
	out.Today, err = p.today(ctx, q)
	if err != nil {
		return out, err
	}
	out.RecentActivity, err = p.activity(ctx, q.Scope)
	return out, err
}

func (p *Postgres) takingNow(ctx context.Context, q domain.HomeQuery) (domain.TakingNow, error) {
	var out domain.TakingNow
	err := p.QueryRow(ctx, `SELECT count(DISTINCT at.student_id),count(DISTINCT at.assignment_id)
 FROM app.attempts at WHERE at.status = 'in_progress' AND at.deadline_at > $3 AND `+ownPapers(), append(homeArgs(q.Scope), q.Now)...).Scan(&out.Students, &out.Assignments)
	return out, err
}
