//go:build integration

package repositories_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"quizzivy/internal/modules/notifications/application/command"
)

func TestThePruneDeletesWhatWasFirstWrittenMoreThan180DaysAgo(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	userID, other := newUser(t, pool, "teacher"), newUser(t, pool, "student")
	now := time.Now().UTC().Truncate(time.Microsecond)
	app.SetClock(func() time.Time { return now })
	day := 24 * time.Hour
	cutoff := now.Add(-180 * day)

	ages := map[string]time.Time{
		"181 days":            now.Add(-181 * day),
		"a moment past 180":   cutoff.Add(-time.Microsecond),
		"exactly 180 days":    cutoff,
		"a moment within 180": cutoff.Add(time.Microsecond),
		"179 days":            now.Add(-179 * day),
		"today":               now,
	}
	kept := []string{"exactly 180 days", "a moment within 180", "179 days", "today"}
	ids := map[string]string{}
	for label, createdAt := range ages {
		for _, owner := range []string{userID, other} {
			var id string
			if err := pool.QueryRow(context.Background(), `
				INSERT INTO app.notifications (user_id, kind, params, dedupe_key, created_at, read_at)
				VALUES ($1::uuid, 'result.ready', '{"title":"Đề giữa kỳ"}'::jsonb, $2, $3, $3)
				RETURNING id::text`, owner, label, createdAt).Scan(&id); err != nil {
				t.Fatalf("%s: %v", label, err)
			}
			if owner == userID {
				ids[id] = label
			}
		}
	}

	deleted, err := app.Commands.Prune.Handle(context.Background(), command.Prune{})
	if err != nil {
		t.Fatal(err)
	}
	if deleted < 4 {
		t.Errorf("the prune reports %d rows, want at least the four this test aged past 180 days", deleted)
	}
	for _, owner := range []string{userID, other} {
		var left []string
		for _, r := range rowsOf(t, pool, owner) {
			if owner == userID {
				left = append(left, ids[r.id])
			} else {
				left = append(left, r.createdAt.UTC().Format(time.RFC3339Nano))
			}
		}
		if owner == userID {
			slices.Sort(left)
			want := slices.Clone(kept)
			slices.Sort(want)
			if !slices.Equal(left, want) {
				t.Errorf("the prune left %v, want %v", left, want)
			}
		} else if len(left) != len(kept) {
			t.Errorf("the prune left another user %d rows, want %d: %v", len(left), len(kept), left)
		}
	}

	again, err := app.Commands.Prune.Handle(context.Background(), command.Prune{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rowsOf(t, pool, userID)) != len(kept) {
		t.Errorf("a second prune removed more rows (%d reported)", again)
	}
}
