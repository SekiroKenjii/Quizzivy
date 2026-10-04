//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/internal/modules/notifications/application"
	"quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/modules/notifications/domain"
)

func mark(t *testing.T, app *application.Application, userID string, ids ...string) {
	t.Helper()
	if _, err := app.Commands.MarkRead.Handle(context.Background(), command.MarkRead{UserID: userID, IDs: ids}); err != nil {
		t.Fatalf("mark read: %v", err)
	}
}

func markAll(t *testing.T, app *application.Application, userID string) {
	t.Helper()
	if _, err := app.Commands.MarkAllRead.Handle(context.Background(), command.MarkAllRead{UserID: userID}); err != nil {
		t.Fatalf("mark all read: %v", err)
	}
}

func TestMarkingReadCountsDownAndKeepsTheFirstReadTime(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	userID := newUser(t, pool, "teacher")
	fill(t, app, userID, 4)
	ids := idsOf(rowsOf(t, pool, userID))
	if got := unread(t, app, userID); got != 4 {
		t.Fatalf("%d unread before anything is marked, want 4", got)
	}

	mark(t, app, userID, ids[0])
	if got := unread(t, app, userID); got != 3 {
		t.Errorf("%d unread after marking one, want 3", got)
	}
	firstRead := rowsOf(t, pool, userID)[0].readAt
	if firstRead == nil {
		t.Fatal("the marked row has no read time")
	}

	time.Sleep(20 * time.Millisecond)
	mark(t, app, userID, ids[0], ids[1], ids[2], ids[1], uuid.NewString())
	rows := rowsOf(t, pool, userID)
	if got := unread(t, app, userID); got != 1 {
		t.Errorf("%d unread after marking three, one of them twice, want 1", got)
	}
	if rows[0].readAt == nil || !rows[0].readAt.Equal(*firstRead) {
		t.Errorf("marking a read row again moved its read time from %v to %v", firstRead, rows[0].readAt)
	}
	if rows[1].readAt == nil || rows[2].readAt == nil || rows[3].readAt != nil {
		t.Errorf("read times are %v, %v and %v, want the second and third read and the fourth not", rows[1].readAt, rows[2].readAt, rows[3].readAt)
	}
	if !rows[1].readAt.After(*firstRead) {
		t.Errorf("the second row was read at %v, not after the first at %v: the clock did not move between the two marks", rows[1].readAt, firstRead)
	}

	time.Sleep(20 * time.Millisecond)
	markAll(t, app, userID)
	after := rowsOf(t, pool, userID)
	if got := unread(t, app, userID); got != 0 {
		t.Errorf("%d unread after marking all, want 0", got)
	}
	for i := range 3 {
		if !after[i].readAt.Equal(*rows[i].readAt) {
			t.Errorf("marking all moved row %d's read time from %v to %v", i, rows[i].readAt, after[i].readAt)
		}
	}
	if after[3].readAt == nil {
		t.Error("marking all left the last row unread")
	}
}

func TestOneCallMarksAtMostAHundredIds(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	userID := newUser(t, pool, "student")
	fill(t, app, userID, 2)
	ids := idsOf(rowsOf(t, pool, userID))
	padded := func(n int) []string {
		out := []string{ids[0]}
		for len(out) < n {
			out = append(out, uuid.NewString())
		}
		return out
	}

	_, err := app.Commands.MarkRead.Handle(context.Background(), command.MarkRead{UserID: userID, IDs: padded(101)})
	if !errors.Is(err, domain.ErrTooManyIDs) {
		t.Errorf("101 ids: err = %v, want ErrTooManyIDs", err)
	}
	if got := unread(t, app, userID); got != 2 {
		t.Errorf("%d unread after a refused call, want both", got)
	}
	_, err = app.Commands.MarkRead.Handle(context.Background(), command.MarkRead{UserID: userID})
	if !errors.Is(err, domain.ErrNoIDs) {
		t.Errorf("no id: err = %v, want ErrNoIDs", err)
	}
	if got := unread(t, app, userID); got != 2 {
		t.Errorf("%d unread after a call with no id, want both: no id must never mean every row", got)
	}
	mark(t, app, userID, padded(100)...)
	if got := unread(t, app, userID); got != 1 {
		t.Errorf("%d unread after a hundred ids, one of them the caller's, want 1", got)
	}
}

func TestOneUserNeverMarksOrCountsAnothers(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	a, b := newUser(t, pool, "teacher"), newUser(t, pool, "student")
	fill(t, app, a, 2)
	fill(t, app, b, 3)
	theirs := idsOf(rowsOf(t, pool, b))

	mark(t, app, a, theirs...)
	if got := unread(t, app, b); got != 3 {
		t.Errorf("after another user marked their ids, the owner has %d unread, want 3", got)
	}
	for _, n := range page(t, app, b, "", 20).Items {
		if n.ReadAt != nil {
			t.Errorf("the owner reads %s as read at %v after another user marked it", n.ID, n.ReadAt)
		}
	}
	if got := unread(t, app, a); got != 2 {
		t.Errorf("marking another user's ids changed the caller's own count to %d, want 2", got)
	}

	markAll(t, app, a)
	if got := unread(t, app, a); got != 0 {
		t.Errorf("%d unread after the caller marked all, want 0", got)
	}
	if got := unread(t, app, b); got != 3 {
		t.Errorf("the caller marking all left another user %d unread, want 3", got)
	}
	for _, r := range rowsOf(t, pool, b) {
		if r.readAt != nil {
			t.Errorf("another user's row %s was marked at %v", r.id, r.readAt)
		}
	}
}
