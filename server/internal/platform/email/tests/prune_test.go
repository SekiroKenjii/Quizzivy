//go:build integration

package email_test

import (
	"context"
	"testing"
	"time"

	"quizzivy/internal/platform/db"
)

func TestPruneRemovesOnlyRowsWhoseOutcomeIsOlderThanTheRetention(t *testing.T) {
	w := newWorld(t, day(5))
	now := time.Now()
	ctx := context.Background()
	age := func(id, column string, by time.Duration) {
		t.Helper()
		if _, err := w.pool.Exec(ctx, `UPDATE app.email_outbox SET `+column+` = $2, secret_ciphertext = NULL WHERE id = $1::uuid`, id, now.Add(-by)); err != nil {
			t.Fatal(err)
		}
	}
	sentOld := w.enqueue(t, invite(addr(t, "sent-old")))
	age(sentOld, "sent_at", 31*24*time.Hour)
	failedOld := w.enqueue(t, invite(addr(t, "failed-old")))
	age(failedOld, "failed_at", 45*24*time.Hour)
	sentRecent := w.enqueue(t, invite(addr(t, "sent-recent")))
	age(sentRecent, "sent_at", 29*24*time.Hour)
	failedRecent := w.enqueue(t, invite(addr(t, "failed-recent")))
	age(failedRecent, "failed_at", 2*24*time.Hour)
	pending := w.enqueue(t, invite(addr(t, "pending")))
	if _, err := w.pool.Exec(ctx, `UPDATE app.email_outbox SET created_at = $2 WHERE id = $1::uuid`, pending, now.Add(-90*24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	removed, err := w.outbox.Prune(ctx, db.NewContext(w.pool), now, 1000)
	if err != nil || removed < 2 {
		t.Fatalf("pruned %d rows (%v), want at least the two old ones", removed, err)
	}
	exists := func(id string) bool {
		t.Helper()
		var n int
		if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM app.email_outbox WHERE id = $1::uuid`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}
	for name, c := range map[string]struct {
		id   string
		kept bool
	}{
		"sent 31 days ago":    {sentOld, false},
		"failed 45 days ago":  {failedOld, false},
		"sent 29 days ago":    {sentRecent, true},
		"failed 2 days ago":   {failedRecent, true},
		"pending for 90 days": {pending, true},
	} {
		if exists(c.id) != c.kept {
			t.Errorf("the row %s: kept = %v, want %v", name, !c.kept, c.kept)
		}
	}

	again, err := w.outbox.Prune(ctx, db.NewContext(w.pool), now, 1000)
	if err != nil || again != 0 {
		t.Errorf("a second prune removed %d rows (%v), want none", again, err)
	}
}

func TestPruneTakesAtMostTheLimitEachTime(t *testing.T) {
	w := newWorld(t, day(6))
	now := time.Now()
	ctx := context.Background()
	var old []string
	for range 3 {
		id := w.enqueue(t, invite(addr(t, "batch")))
		if _, err := w.pool.Exec(ctx, `UPDATE app.email_outbox SET sent_at = $2 WHERE id = $1::uuid`, id, now.Add(-40*24*time.Hour)); err != nil {
			t.Fatal(err)
		}
		old = append(old, id)
	}
	first, err := w.outbox.Prune(ctx, db.NewContext(w.pool), now, 2)
	if err != nil || first != 2 {
		t.Fatalf("first batch removed %d rows (%v), want exactly the limit, 2", first, err)
	}
	for {
		n, err := w.outbox.Prune(ctx, db.NewContext(w.pool), now, 2)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	var left int
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM app.email_outbox WHERE id = ANY($1::uuid[])`, old).Scan(&left); err != nil || left != 0 {
		t.Errorf("%d old rows left (%v), want none", left, err)
	}
}

func TestPruneNeedsAPositiveLimit(t *testing.T) {
	w := newWorld(t, day(7))
	if _, err := w.outbox.Prune(context.Background(), db.NewContext(w.pool), time.Now(), 0); err == nil {
		t.Error("a limit of 0 was accepted")
	}
}
