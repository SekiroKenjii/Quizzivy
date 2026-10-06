package domain

import (
	"context"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/paging"
	"time"
)

// Repository reads the dashboard's aggregates over what a scope reaches:
// the assignments visibility.AssignmentIDs gives, the papers
// visibility.Papers shows and the classes the scope teaches, or every row
// under scope.all. A zero scope reads nothing.
type Repository interface {
	Summary(ctx context.Context, q SummaryQuery) (Summary, error)
	Home(ctx context.Context, q HomeQuery) (Home, error)
	LiveAssignments(ctx context.Context, scope access.Scope, now time.Time) (int, error)
	AnswersToGrade(ctx context.Context, scope access.Scope) (int, error)
	List(ctx context.Context, q ListQuery) ([]Recent, paging.Page, error)
}
