package repositories

import (
	"context"
	"quizzivy/internal/shared/access"
	"time"
)

// LiveAssignments counts the same open state as the Assignments list at the supplied clock.
func (p *Postgres) LiveAssignments(ctx context.Context, scope access.Scope, now time.Time) (int, error) {
	var count int
	err := p.QueryRow(ctx, `SELECT count(*) FROM app.assignments a WHERE `+ownAssignments()+`
 AND a.published_at IS NOT NULL AND (a.closed_at IS NULL OR a.closed_at > $3)
 AND a.opens_at <= $3 AND a.closes_at > $3`, append(homeArgs(scope), now)...).Scan(&count)
	return count, err
}

// AnswersToGrade counts unmarked manual answers in reachable handed-in papers.
func (p *Postgres) AnswersToGrade(ctx context.Context, scope access.Scope) (int, error) {
	var count int
	err := p.QueryRow(ctx, `SELECT count(*) FROM app.attempts at
 JOIN app.attempt_answers ans ON ans.attempt_id=at.id
 WHERE `+ownPapers()+` AND at.status IN ('submitted','timed_out')
 AND ans.requires_manual AND ans.manual_score IS NULL`, homeArgs(scope)...).Scan(&count)
	return count, err
}
