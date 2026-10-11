package domain_test

import (
	"testing"
	"time"

	"quizzivy/internal/modules/assignments/domain"
)

func TestASaveReopensOnlyWhatItTurnsFromClosedToNotClosed(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	published := now.Add(-72 * time.Hour)
	early := now.Add(-30 * time.Minute)
	windowOpens, windowCloses := now.Add(-3*time.Hour), now.Add(-time.Hour)

	saving := func(opens, closes time.Time, closeNow bool) domain.WriteInput {
		return domain.WriteInput{OpensAt: opens, ClosesAt: closes, CloseNow: closeNow, Now: now}
	}

	for _, tc := range []struct {
		name      string
		published *time.Time
		closedAt  *time.Time
		opens     time.Time
		closes    time.Time
		in        domain.WriteInput
		want      bool
	}{
		{"a window-closed assignment saved with its own window", &published, nil, windowOpens, windowCloses, saving(windowOpens, windowCloses, false), false},
		{"a window-closed assignment given an earlier close", &published, nil, windowOpens, windowCloses, saving(windowOpens, windowCloses.Add(-time.Minute), false), false},
		{"a window-closed assignment given a close that is still past", &published, nil, windowOpens, windowCloses, saving(windowOpens, now.Add(-time.Nanosecond), false), false},
		{"a window-closed assignment given a close exactly now", &published, nil, windowOpens, windowCloses, saving(windowOpens, now, false), false},
		{"a window-closed assignment given a close a nanosecond ahead", &published, nil, windowOpens, windowCloses, saving(windowOpens, now.Add(time.Nanosecond), false), true},
		{"a window-closed assignment given a later close", &published, nil, windowOpens, windowCloses, saving(windowOpens, now.Add(time.Hour), false), true},
		{"a window-closed assignment given a window that opens later", &published, nil, windowOpens, windowCloses, saving(now.Add(time.Hour), now.Add(2*time.Hour), false), true},
		{"a window-closed assignment closed now beside a later close", &published, nil, windowOpens, windowCloses, saving(windowOpens, now.Add(time.Hour), true), false},
		{"a window-closed assignment closed now beside a window that opens later", &published, nil, windowOpens, windowCloses, saving(now.Add(time.Hour), now.Add(2*time.Hour), true), true},
		{"an assignment closed early saved with a later close", &published, &early, windowOpens, now.Add(time.Hour), saving(windowOpens, now.Add(3*time.Hour), false), false},
		{"an assignment closed early saved with a close in the past", &published, &early, windowOpens, now.Add(time.Hour), saving(windowOpens, windowCloses, false), false},
		{"an assignment closed early saved with a window that opens after the early close", &published, &early, windowOpens, now.Add(time.Hour), saving(early.Add(time.Nanosecond), now.Add(3*time.Hour), false), true},
		{"an assignment closed early saved with a window that opens at the early close", &published, &early, windowOpens, now.Add(time.Hour), saving(early, now.Add(3*time.Hour), false), false},
		{"an open assignment given a later close", &published, nil, now.Add(-time.Hour), now.Add(time.Hour), saving(now.Add(-time.Hour), now.Add(3*time.Hour), false), false},
		{"an open assignment given a window in the past", &published, nil, now.Add(-time.Hour), now.Add(time.Hour), saving(windowOpens, windowCloses, false), false},
		{"a scheduled assignment given a later window", &published, nil, now.Add(time.Hour), now.Add(2 * time.Hour), saving(now.Add(2*time.Hour), now.Add(3*time.Hour), false), false},
		{"a draft given any window", nil, nil, windowOpens, windowCloses, saving(now.Add(time.Hour), now.Add(2*time.Hour), false), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.Schedule.Reopens(tc.published, tc.closedAt, tc.opens, tc.closes, tc.in); got != tc.want {
				t.Errorf("Reopens = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAnEarlyCloseIsKeptAndNeverLiftedBySaving(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	held := now.Add(-time.Hour)

	if got := domain.Schedule.ClosedAtAfter(&held, domain.WriteInput{Now: now}); got == nil || !got.Equal(held) {
		t.Errorf("an early close was %v after a save that did not mention it, want it kept", got)
	}
	if got := domain.Schedule.ClosedAtAfter(&held, domain.WriteInput{Now: now, CloseNow: true}); got == nil || !got.Equal(held) {
		t.Errorf("closing now moved the early close to %v, want the first one kept", got)
	}
	if got := domain.Schedule.ClosedAtAfter(nil, domain.WriteInput{Now: now, CloseNow: true}); got == nil || !got.Equal(now) {
		t.Errorf("closing now gave %v, want now", got)
	}
	if got := domain.Schedule.ClosedAtAfter(nil, domain.WriteInput{Now: now}); got != nil {
		t.Errorf("a save that closes nothing gave %v, want none", got)
	}
}
