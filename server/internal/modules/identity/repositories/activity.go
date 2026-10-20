package repositories

import (
	"context"
	"fmt"
	"time"

	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/db"
)

// Activity writes what users did: their last_active_at and the days they were active.
type Activity struct {
	db.Repository
}

func NewActivity(dbx db.Context) *Activity { return &Activity{Repository: db.NewRepository(dbx)} }

var _ domain.Activity = (*Activity)(nil)

// RecordActivity raises each user's last_active_at to the newest entry given for them, in id order so two instances flushing together take row locks in the same order, and adds the days with ON CONFLICT DO NOTHING. A user who no longer exists drops out of both statements.
func (a *Activity) RecordActivity(ctx context.Context, entries []domain.ActivityEntry) error {
	if len(entries) == 0 {
		return nil
	}
	ids := make([]string, len(entries))
	ats := make([]time.Time, len(entries))
	days := make([]time.Time, len(entries))
	for i, e := range entries {
		ids[i], ats[i], days[i] = e.UserID, e.At, e.Day
	}
	tx, err := a.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin activity: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const touch = `
		UPDATE app.users u
		   SET last_active_at = greatest(u.last_active_at, newest.at)
		  FROM (SELECT e.user_id, max(e.at) AS at
		          FROM unnest($1::uuid[], $2::timestamptz[]) AS e(user_id, at)
		         GROUP BY e.user_id
		         ORDER BY e.user_id) AS newest
		 WHERE u.id = newest.user_id`
	if _, err := tx.Exec(ctx, touch, ids, ats); err != nil {
		return fmt.Errorf("stamp last active: %w", err)
	}
	const record = `
		INSERT INTO app.user_activity_days (user_id, day)
		SELECT DISTINCT e.user_id, e.day
		  FROM unnest($1::uuid[], $2::date[]) AS e(user_id, day)
		  JOIN app.users u ON u.id = e.user_id
		ON CONFLICT DO NOTHING`
	if _, err := tx.Exec(ctx, record, ids, days); err != nil {
		return fmt.Errorf("record activity days: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit activity: %w", err)
	}
	return nil
}

// PruneDays deletes at most limit days earlier than before, found on the day index.
func (a *Activity) PruneDays(ctx context.Context, before time.Time, limit int) (int, error) {
	tag, err := a.Exec(ctx, `
		DELETE FROM app.user_activity_days d
		 USING (SELECT user_id, day FROM app.user_activity_days WHERE day < $1::date ORDER BY day LIMIT $2) old
		 WHERE d.user_id = old.user_id AND d.day = old.day`, before, limit)
	if err != nil {
		return 0, fmt.Errorf("prune activity days: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
