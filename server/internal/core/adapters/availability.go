package adapters

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	attemptsdomain "quizzivy/internal/modules/attempts/domain"
	availabilityquery "quizzivy/internal/modules/availability/application/query"
	availabilitydomain "quizzivy/internal/modules/availability/domain"
	availabilityrepo "quizzivy/internal/modules/availability/repositories"
	"quizzivy/internal/shared/cqrs"
)

// AttemptStartGuard runs availability's maintenance check inside the
// transaction that starts an attempt.
type AttemptStartGuard struct{}

// GuardAttemptStart returns the window an attempt from now until deadline
// would run into, if any.
func (AttemptStartGuard) GuardAttemptStart(ctx context.Context, tx pgx.Tx, now, deadline time.Time) (*attemptsdomain.MaintenanceWindow, error) {
	window, err := availabilityrepo.GuardAttemptStart(ctx, tx, now, deadline)
	if err != nil || window == nil {
		return nil, err
	}
	return &attemptsdomain.MaintenanceWindow{StartsAt: window.StartsAt, EndsAt: window.EndsAt}, nil
}

// MaintenanceGate answers the HTTP maintenance gate from availability's
// CurrentWindow query.
type MaintenanceGate struct {
	Current cqrs.QueryHandler[availabilityquery.CurrentWindow, availabilitydomain.Status]
}

// ActiveWindow returns the window under way now, if any.
func (g MaintenanceGate) ActiveWindow(ctx context.Context) (time.Time, time.Time, bool) {
	status, err := g.Current.Handle(ctx, availabilityquery.CurrentWindow{})
	if err != nil || !status.Active || status.Window == nil {
		return time.Time{}, time.Time{}, false
	}
	return status.Window.StartsAt, status.Window.EndsAt, true
}
