//go:build integration

package email_test

import (
	"context"
	"testing"
	"time"

	"quizzivy/internal/platform/email"
)

func mailFor(userID, address string) email.Mail {
	m := invite(address)
	m.ToUserID = userID
	return m
}

func TestCancelPendingEndsOnlyTheMessagesStillWaitingForThatUser(t *testing.T) {
	w := newWorld(t, day(8))
	ctx := context.Background()
	user, other := newUser(t, w.pool, "vi"), newUser(t, w.pool, "vi")
	waiting := w.enqueue(t, mailFor(user, addr(t, "waiting")))
	later := w.enqueue(t, mailFor(user, addr(t, "later")))
	sent := w.enqueue(t, mailFor(user, addr(t, "sent")))
	theirs := w.enqueue(t, mailFor(other, addr(t, "theirs")))
	at := day(8)
	if _, err := w.pool.Exec(ctx, `UPDATE app.email_outbox SET sent_at = $2 WHERE id = $1::uuid`, sent, at.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	before := read(t, w.pool, sent)

	cancelled, err := email.CancelPending(ctx, w.pool, user, at)
	if err != nil || cancelled != 2 {
		t.Fatalf("cancelled %d (%v), want the two waiting messages", cancelled, err)
	}

	for name, id := range map[string]string{"waiting": waiting, "later": later} {
		r := read(t, w.pool, id)
		if r.failedAt == nil || !r.failedAt.Equal(at) || r.lastError != "cancelled" || r.hasSecret || r.sentAt != nil || r.attempts != 0 {
			t.Errorf("%s message = %+v, want it failed as cancelled at the time given with its secret erased", name, r)
		}
	}
	if after := read(t, w.pool, sent); after.failedAt != nil || after.sentAt == nil || !after.sentAt.Equal(*before.sentAt) || after.lastError != before.lastError {
		t.Errorf("the sent message went from %+v to %+v", before, after)
	}
	if r := read(t, w.pool, theirs); r.failedAt != nil || r.lastError != "" || !r.hasSecret {
		t.Errorf("another user's message = %+v, want it untouched", r)
	}

	if again, err := email.CancelPending(ctx, w.pool, user, at); err != nil || again != 0 {
		t.Errorf("a second call cancelled %d (%v), want none", again, err)
	}
	if _, err := w.disp.Dispatch(ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{waiting, later} {
		if r := read(t, w.pool, id); r.sentAt != nil {
			t.Errorf("a cancelled message %s was sent", id)
		}
	}
}

func TestCancelPendingDoesNotWaitForAMessageADispatcherHoldsLocked(t *testing.T) {
	w := newWorld(t, day(9))
	ctx := context.Background()
	user := newUser(t, w.pool, "vi")
	held := w.enqueue(t, mailFor(user, addr(t, "held")))
	free := w.enqueue(t, mailFor(user, addr(t, "free")))

	holder, err := w.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	if _, err := holder.Exec(ctx, `SELECT 1 FROM app.email_outbox WHERE id = $1::uuid FOR UPDATE`, held); err != nil {
		t.Fatal(err)
	}

	within, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cancelled, err := email.CancelPending(within, w.pool, user, day(9))
	if err != nil || cancelled != 1 {
		t.Fatalf("cancelled %d (%v), want only the message nobody holds, without waiting for the other", cancelled, err)
	}
	if r := read(t, w.pool, free); r.lastError != "cancelled" {
		t.Errorf("the free message = %+v, want it cancelled", r)
	}
}
