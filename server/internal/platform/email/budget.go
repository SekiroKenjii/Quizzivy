package email

import (
	"context"
	"fmt"
	"time"

	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/limits"
)

// Budget answers how much of the day's sending is left. Limit and
// MonthlyLimit are the most messages sent in one local day and one local month
// of Location, as the Dispatcher's; one that is zero is read from the usage
// limits resolved under Limits.
type Budget struct {
	DB           db.Context
	Limit        int
	MonthlyLimit int
	Limits       limits.Env
	Location     *time.Location
}

// Remaining returns the day's limit less what has been sent in the local day
// of now and what waits to be sent in it, never below zero and never above
// what the month has left counted the same way, and the day's limit. A message
// waiting for a later day, or failed, does not count.
func (b Budget) Remaining(ctx context.Context, now time.Time) (remaining, limit int, err error) {
	daily, monthly, err := caps(ctx, b.DB, b.Limits, b.Limit, b.MonthlyLimit)
	if err != nil {
		return 0, 0, err
	}
	dayStart, nextDay := localDay(now, b.Location)
	monthStart, nextMonth := localMonth(now, b.Location)
	const waiting = `(sent_at IS NULL AND failed_at IS NULL AND deliver_after < $2 AND template = ANY($5::text[]))`
	var today, month int
	if err := b.DB.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE (sent_at >= $1 AND sent_at < $2) OR `+waiting+`),
		       count(*) FILTER (WHERE (sent_at >= $3 AND sent_at < $4) OR `+waiting+`)
		  FROM app.email_outbox
		 WHERE (sent_at >= $3 AND sent_at < $4) OR `+waiting,
		dayStart, nextDay, monthStart, nextMonth, templateNames()).Scan(&today, &month); err != nil {
		return 0, 0, fmt.Errorf("email: count the day's messages: %w", err)
	}
	return max(0, min(daily-today, monthly-month)), daily, nil
}
