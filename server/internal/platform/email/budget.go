package email

import (
	"context"
	"fmt"
	"time"

	"quizzivy/internal/platform/db"
)

// Budget answers how much of the day's sending is left. Limit is the most
// messages sent in one local day of Location, as the Dispatcher's DailyLimit.
type Budget struct {
	DB       db.Context
	Limit    int
	Location *time.Location
}

// Remaining returns Limit less what has been sent in the local day of now and
// what waits to be sent in it, never below zero, and Limit. A message waiting
// for a later day, or failed, does not count.
func (b Budget) Remaining(ctx context.Context, now time.Time) (remaining, limit int, err error) {
	dayStart, nextDay := localDay(now, b.Location)
	var used int
	if err := b.DB.QueryRow(ctx, `
		SELECT count(*) FROM app.email_outbox
		 WHERE (sent_at >= $1 AND sent_at < $2)
		    OR (sent_at IS NULL AND failed_at IS NULL AND deliver_after < $2 AND template = ANY($3::text[]))`,
		dayStart, nextDay, templateNames()).Scan(&used); err != nil {
		return 0, 0, fmt.Errorf("email: count the day's messages: %w", err)
	}
	return max(0, b.Limit-used), b.Limit, nil
}
