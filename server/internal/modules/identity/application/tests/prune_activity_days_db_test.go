//go:build integration

package application_test

import (
	"context"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
)

func TestPruningActivityRemovesOnlyDaysPastTheLimitAndInBatches(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	zone := time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	svc.SetActivity(repositories.NewActivity(db.NewContext(pool)), zone, nil)
	id, _ := makeUser(t, pool)
	ctx := context.Background()

	local := time.Now().In(zone)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	for _, age := range []int{500, 402, 401, 400, 399, 0} {
		if _, err := pool.Exec(ctx, `INSERT INTO app.user_activity_days (user_id, day) VALUES ($1, $2)`, id, today.AddDate(0, 0, -age)); err != nil {
			t.Fatal(err)
		}
	}
	prune := func(limit int) int {
		t.Helper()
		n, err := svc.Commands.PruneActivityDays.Handle(ctx, command.PruneActivityDays{Limit: limit})
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if got := prune(2); got != 2 {
		t.Errorf("first batch removed %d, want 2", got)
	}
	if got := prune(2); got != 1 {
		t.Errorf("second batch removed %d, want the 1 left", got)
	}
	if got := prune(2); got != 0 {
		t.Errorf("a third batch removed %d, want 0", got)
	}
	var kept int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.user_activity_days WHERE user_id = $1`, id).Scan(&kept); err != nil || kept != 3 {
		t.Errorf("%d days kept (%v), want the 400-day-old one, the 399-day-old one and today", kept, err)
	}
	var oldest time.Time
	if err := pool.QueryRow(ctx, `SELECT min(day) FROM app.user_activity_days WHERE user_id = $1`, id).Scan(&oldest); err != nil || !oldest.Equal(today.AddDate(0, 0, -400)) {
		t.Errorf("oldest kept day %v (%v), want exactly 400 days back", oldest, err)
	}
}
