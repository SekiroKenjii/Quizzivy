package domain

import (
	"context"
	"time"
)

// Repository stores the pool, the allowances, the reservations, the ledger and the requests. Every method that moves a credit does it in one transaction that takes the month's row before the teacher's, so no two can deadlock, and the database's constraints refuse what would overspend.
type Repository interface {
	// Reserve holds in.Credits against the month's pool and the teacher's allowance, creating either row the month's first use needs, and returns the reservation. It answers ErrPaused for a paused teacher before it takes any lock, and *Exhausted when the allowance or the pool cannot cover the credits, in which case it writes nothing.
	Reserve(ctx context.Context, in ReserveInput) (Reservation, error)
	// Settle ends the reservation: it moves Charged credits from reserved to used on both rows, deletes the reservation and appends the ledger row, in one transaction. It answers ErrReservationGone when the reservation is not held and writes nothing then.
	Settle(ctx context.Context, s Settlement) (Settled, error)
	// Release deletes the reservation and returns its credits, and reports whether it was held.
	Release(ctx context.Context, r Reservation) (bool, error)
	// ReleaseStale returns every reservation created before cutoff.
	ReleaseStale(ctx context.Context, cutoff time.Time) (Released, error)
	// Usage reads one teacher's month and creates nothing: a teacher with no row yet reads the allowance a first use would give.
	Usage(ctx context.Context, month time.Time, userID string) (MonthUsage, error)
	// Admin reads the month for the Admin page, with the rules in force.
	Admin(ctx context.Context, month time.Time) (AdminCredits, error)
	// Rules reads the centre's rules.
	Rules(ctx context.Context) (Rules, error)
	// SetAllowance changes a teacher's allowance or pause for the month, creating the row if need be, and writes the audit row in the same statement. It answers ErrNotATeacher for a user who is not one.
	SetAllowance(ctx context.Context, in SetAllowanceInput) (TeacherMonth, error)
	// AddCredits adds to the month's pool and writes the audit row in the same statement.
	AddCredits(ctx context.Context, in AddCreditsInput) (Pool, error)
	// SetRules changes the rules and, when the default allowance changed, rewrites the month's non-custom allowances, never below what is already used or held; the audit row is written in the same statement.
	SetRules(ctx context.Context, in SetRulesInput) (RulesChange, error)
	// CreateRequest records a request, answering ErrRequestsOff while the rule is to pause, ErrRequestPending while one is waiting and ErrNotATeacher for a user who is not one.
	CreateRequest(ctx context.Context, in CreateRequestInput) (Filed, error)
	// RequestOwner reads whose request it is, whatever its status, and answers ErrRequestNotFound for an id that is none.
	RequestOwner(ctx context.Context, id string) (string, error)
	// DecideRequest approves or declines a pending request and writes the audit row in the same statement. An approval raises the teacher's allowance for the request's month by the amount.
	DecideRequest(ctx context.Context, in DecideInput) (Decided, error)
}
