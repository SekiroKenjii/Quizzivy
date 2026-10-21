package email

import (
	"context"
	"math"
	"time"

	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/limits"
)

// caps returns the most messages one local day and one local month may send:
// daily and monthly when positive, and otherwise the usage limits resolved on
// q under env, with no cap where none is set.
func caps(ctx context.Context, q db.Querier, env limits.Env, daily, monthly int) (int, int, error) {
	if daily > 0 && monthly > 0 {
		return daily, monthly, nil
	}
	resolved, err := limits.Read(ctx, q, env)
	if err != nil {
		return 0, 0, err
	}
	of := func(n limits.Name) int {
		if v, ok := resolved.Of(n); ok {
			return int(v)
		}
		return math.MaxInt32
	}
	if daily <= 0 {
		daily = of(limits.EmailsPerDay)
	}
	if monthly <= 0 {
		monthly = of(limits.EmailsPerMonth)
	}
	return daily, monthly, nil
}

func localMonth(now time.Time, location *time.Location) (time.Time, time.Time) {
	if location == nil {
		location = time.UTC
	}
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
	return start, start.AddDate(0, 1, 0)
}

// failExpired ends every waiting message whose link expired by now as a failed
// one with the code "expired" and its sealed secret erased (D-060), skipping a
// row another dispatcher holds, and reports how many it ended.
func failExpired(ctx context.Context, q db.Querier, now time.Time) (int, error) {
	tag, err := q.Exec(ctx, `
		UPDATE app.email_outbox o
		   SET failed_at = $1, last_error_code = 'expired', secret_ciphertext = NULL
		 WHERE o.id IN (SELECT w.id FROM app.email_outbox w
		                 WHERE w.sent_at IS NULL AND w.failed_at IS NULL AND w.expires_at <= $1
		                   FOR UPDATE SKIP LOCKED)`, now)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
