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
	AdminRepository
}

// AdminRepository counts the whole centre for the Admin console, over every
// row and under no scope: the caller's keys decide what it may read, and the
// application asks only for that.
type AdminRepository interface {
	// Accounts counts the users in one pass; since is the start of the month
	// whose new students StudentsNew counts.
	Accounts(ctx context.Context, since time.Time) (Accounts, error)
	// MustChangePassword counts the enabled accounts on a temporary password.
	MustChangePassword(ctx context.Context) (int, error)
	// ActiveBetween counts the distinct accounts with an activity row on any
	// day from..to.
	ActiveBetween(ctx context.Context, from, to time.Time) (int, error)
	// ActiveByDay counts, for each day from..to with an activity row, the
	// student targets and the other accounts active on it.
	ActiveByDay(ctx context.Context, from, to time.Time) ([]ActiveDay, error)
	ClassCounts(ctx context.Context) (ClassCounts, error)
	DisabledTeacherClasses(ctx context.Context) (DisabledTeacherClasses, error)
	// Held sums what the media library, the import documents and the attempts
	// keep.
	Held(ctx context.Context) (Held, error)
}
