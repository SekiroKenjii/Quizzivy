//go:build integration

package email_test

import (
	"context"
	"testing"
	"time"

	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/email"
)

func longAgo(n int) time.Time {
	return time.Date(2001, 3, n, 12, 0, 0, 0, time.FixedZone("ICT", 7*3600))
}

func waiting(t *testing.T, w *world, at time.Time, label string) string {
	t.Helper()
	return w.enqueue(t, email.Mail{To: addr(t, label), Params: email.SignInInviteParams{FullName: "An"}, DeliverAfter: at.Add(-time.Minute)})
}

func (w *world) budget(limit int) email.Budget {
	return email.Budget{DB: db.NewContext(w.pool), Limit: limit, Location: time.FixedZone("ICT", 7*3600)}
}

func remaining(t *testing.T, b email.Budget, at time.Time) (int, int) {
	t.Helper()
	left, limit, err := b.Remaining(context.Background(), at)
	if err != nil {
		t.Fatal(err)
	}
	return left, limit
}

func TestTheBudgetCountsWhatWaitsAndWhatWasSentInTheLocalDay(t *testing.T) {
	at := longAgo(20)
	w := newWorld(t, at)
	b := w.budget(5)
	startLeft, limit := remaining(t, b, at)
	if limit != 5 || startLeft != 5 {
		t.Fatalf("an empty day has %d of %d left, want 5 of 5", startLeft, limit)
	}

	for range 2 {
		waiting(t, w, at, "cho")
	}
	if left, _ := remaining(t, b, at); left != 3 {
		t.Errorf("two messages waiting leave %d, want 3", left)
	}

	if _, err := w.disp.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if left, _ := remaining(t, b, at); left != 3 {
		t.Errorf("two messages sent leave %d, want the same 3: a sent message was a waiting one", left)
	}
}

func TestARetryForAnotherDayOrAMessageThatFailedUsesNoneOfToday(t *testing.T) {
	at := longAgo(21)
	w := newWorld(t, at)
	b := w.budget(5)

	retry := w.enqueue(t, email.Mail{To: addr(t, "mai"), Params: email.SignInInviteParams{FullName: "An"}, DeliverAfter: at.AddDate(0, 0, 1)})
	if _, err := w.pool.Exec(context.Background(), `UPDATE app.email_outbox SET attempts = 1, last_error_code = 'unavailable' WHERE id = $1::uuid`, retry); err != nil {
		t.Fatal(err)
	}
	id := waiting(t, w, at, "hong")
	if _, err := w.pool.Exec(context.Background(), `UPDATE app.email_outbox SET failed_at = $2, last_error_code = 'rejected' WHERE id = $1::uuid`, id, at); err != nil {
		t.Fatal(err)
	}

	if left, _ := remaining(t, b, at); left != 5 {
		t.Errorf("a retry for tomorrow and a failed message leave %d, want 5", left)
	}
	if left, _ := remaining(t, b, at.AddDate(0, 0, 1)); left != 4 {
		t.Errorf("tomorrow's retry leaves %d on its own day, want 4", left)
	}
}

func TestMailACapHeldUsesTodaysBudgetSinceItGoesFirst(t *testing.T) {
	at := longAgo(23)
	w := newWorld(t, at)
	w.enqueue(t, email.Mail{To: addr(t, "giu"), Params: email.SignInInviteParams{FullName: "An"}, DeliverAfter: at.AddDate(0, 1, 0)})
	if left, limit := remaining(t, w.budget(5), at); left != 4 || limit != 5 {
		t.Errorf("one held message leaves %d of %d, want 4 of 5", left, limit)
	}
	monthly := w.budget(5)
	monthly.MonthlyLimit = 3
	if left, limit := remaining(t, monthly, at); left != 2 || limit != 3 {
		t.Errorf("with the month's cap at 3 the day has %d of %d, want 2 of the month's 3", left, limit)
	}
}

func TestTheBudgetNeverGoesBelowZeroAndTheDayRollsAtLocalMidnight(t *testing.T) {
	at := longAgo(22)
	w := newWorld(t, at)
	for range 3 {
		waiting(t, w, at, "du")
	}
	if left, _ := remaining(t, w.budget(2), at); left != 0 {
		t.Errorf("three messages against a limit of two leave %d, want 0", left)
	}

	lateEvening := time.Date(2001, 3, 22, 23, 59, 0, 0, time.FixedZone("ICT", 7*3600))
	justAfter := lateEvening.Add(2 * time.Minute)
	if _, err := w.pool.Exec(context.Background(),
		`UPDATE app.email_outbox SET sent_at = $2 WHERE id = ANY($1::uuid[])`, w.ids, lateEvening); err != nil {
		t.Fatal(err)
	}
	if left, _ := remaining(t, w.budget(3), lateEvening); left != 0 {
		t.Errorf("at 23:59 the day has %d left, want 0", left)
	}
	if left, _ := remaining(t, w.budget(3), justAfter); left != 3 {
		t.Errorf("two minutes later the day has %d left, want a fresh 3", left)
	}
}

func TestADayOfTwentyThreeOrTwentyFiveHoursStartsAndEndsAtLocalMidnight(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		day, next time.Time
		lastOfDay time.Time
		firstNext time.Time
	}{
		"clocks go forward, a day of 23 hours": {
			day:       time.Date(2001, 3, 25, 12, 0, 0, 0, berlin),
			next:      time.Date(2001, 3, 26, 12, 0, 0, 0, berlin),
			lastOfDay: time.Date(2001, 3, 25, 23, 30, 0, 0, berlin),
			firstNext: time.Date(2001, 3, 26, 0, 30, 0, 0, berlin),
		},
		"clocks go back, a day of 25 hours": {
			day:       time.Date(2001, 10, 28, 12, 0, 0, 0, berlin),
			next:      time.Date(2001, 10, 29, 12, 0, 0, 0, berlin),
			lastOfDay: time.Date(2001, 10, 28, 23, 30, 0, 0, berlin),
			firstNext: time.Date(2001, 10, 29, 0, 30, 0, 0, berlin),
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t, c.day)
			b := email.Budget{DB: db.NewContext(w.pool), Limit: 5, Location: berlin}
			last, first := waiting(t, w, c.day, "cuoi"), waiting(t, w, c.day, "dau")
			for id, at := range map[string]time.Time{last: c.lastOfDay, first: c.firstNext} {
				if _, err := w.pool.Exec(context.Background(), `UPDATE app.email_outbox SET sent_at = $2 WHERE id = $1::uuid`, id, at); err != nil {
					t.Fatal(err)
				}
			}

			if left, _ := remaining(t, b, c.day); left != 4 {
				t.Errorf("the day itself has %d left, want 4: only the message sent at %s belongs to it", left, c.lastOfDay.Format("15:04"))
			}
			if left, _ := remaining(t, b, c.next); left != 4 {
				t.Errorf("the next day has %d left, want 4: only the message sent at %s belongs to it", left, c.firstNext.Format("15:04"))
			}
		})
	}
}
