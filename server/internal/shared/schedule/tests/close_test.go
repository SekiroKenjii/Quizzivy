package schedule_test

import (
	"quizzivy/internal/shared/schedule"
	"strings"
	"testing"
	"time"
)

var (
	opens  = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	closes = time.Date(2026, 10, 8, 20, 0, 0, 0, time.UTC)
)

func at(offset time.Duration) *time.Time {
	moment := closes.Add(offset)
	return &moment
}

func TestAnAssignmentClosesWhenItsWindowEndsOrItsEarlyCloseComes(t *testing.T) {
	cases := []struct {
		name     string
		closedAt *time.Time
		want     time.Time
	}{
		{"no early close", nil, closes},
		{"an early close before the window ends", at(-48 * time.Hour), closes.Add(-48 * time.Hour)},
		{"closing an assignment whose window already ended", at(3 * time.Hour), closes},
		{"an early close at the end of the window", at(0), closes},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := schedule.Close(closes, c.closedAt, nil); !got.Equal(c.want) {
				t.Errorf("Close = %v, want %v", got, c.want)
			}
		})
	}
}

func TestAnOverrideNeverClosesAnyoneSoonerButMayLetThemInLonger(t *testing.T) {
	cases := []struct {
		name     string
		closedAt *time.Time
		override *time.Time
		want     time.Time
	}{
		{"a later override", nil, at(24 * time.Hour), closes.Add(24 * time.Hour)},
		{"an earlier override is ignored", nil, at(-24 * time.Hour), closes},
		{"an override beats an early close", at(-48 * time.Hour), at(-24 * time.Hour), closes.Add(-24 * time.Hour)},
		{"an override beats an early close and the window", at(-48 * time.Hour), at(5 * time.Hour), closes.Add(5 * time.Hour)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := schedule.Close(closes, c.closedAt, c.override); !got.Equal(c.want) {
				t.Errorf("Close = %v, want %v", got, c.want)
			}
		})
	}
}

func TestTheCloseExpressionNamesTheOverrideOnlyWhenThereIsOne(t *testing.T) {
	plain := schedule.CloseOf("")
	if strings.Contains(plain, "greatest") || !strings.Contains(plain, "a.closed_at") || !strings.Contains(plain, "a.closes_at") {
		t.Errorf("CloseOf(\"\") = %q, want the assignment's own two dates and no override", plain)
	}
	got := schedule.CloseOf("o")
	if !strings.Contains(got, "greatest(") || !strings.Contains(got, "o.closes_at") {
		t.Errorf("CloseOf(\"o\") = %q, want the later of the close and o.closes_at", got)
	}
}

func TestAWindowWithoutAnOverrideIsUnchanged(t *testing.T) {
	w := schedule.Window{OpensAt: opens, ClosesAt: closes, DurationMin: 45, MaxAttempts: 2}
	if got := w.WithOverride(nil); got != w {
		t.Errorf("WithOverride(nil) = %+v, want %+v", got, w)
	}
	if got := w.WithOverride(&schedule.Override{}); got != w {
		t.Errorf("an override that changes nothing = %+v, want %+v", got, w)
	}
}

func TestAnOverrideLengthensTheWindowAndAddsAttempts(t *testing.T) {
	w := schedule.Window{OpensAt: opens, ClosesAt: closes, DurationMin: 45, MaxAttempts: 2}
	minutes := 90
	later := closes.Add(2 * time.Hour)
	got := w.WithOverride(&schedule.Override{ClosesAt: &later, DurationMin: &minutes, ExtraAttempts: 3})
	want := schedule.Window{OpensAt: opens, ClosesAt: later, DurationMin: 90, MaxAttempts: 5}
	if got != want {
		t.Errorf("WithOverride = %+v, want %+v", got, want)
	}
	earlier := closes.Add(-2 * time.Hour)
	if got := w.WithOverride(&schedule.Override{ClosesAt: &earlier}); !got.ClosesAt.Equal(closes) {
		t.Errorf("an earlier override moved the close to %v, want it kept at %v", got.ClosesAt, closes)
	}
	if !w.ClosesAt.Equal(closes) || w.MaxAttempts != 2 {
		t.Errorf("WithOverride changed its receiver: %+v", w)
	}
}
