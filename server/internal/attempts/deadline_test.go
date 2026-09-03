package attempts_test

import (
	"testing"
	"time"

	"quizzivy/internal/attempts"
)

func TestTheDeadlineIsTheEarlierOfDurationAndClose(t *testing.T) {
	now := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		duration int
		closesIn time.Duration
		want     time.Duration
	}{
		{"the whole duration fits before the close", 60, 3 * time.Hour, 60 * time.Minute},
		{"the close cuts a 60-minute test short", 60, 10 * time.Minute, 10 * time.Minute},
		{"they land on the same instant", 30, 30 * time.Minute, 30 * time.Minute},
		{"one minute of a long window", 1, 8 * time.Hour, time.Minute},
		{"the longest test the schema allows", 600, 24 * time.Hour, 600 * time.Minute},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := attempts.Rules{DurationMinutes: c.duration, ClosesAt: now.Add(c.closesIn)}
			if got := r.Deadline(now).Sub(now); got != c.want {
				t.Errorf("deadline %v after start, want %v", got, c.want)
			}
		})
	}
}

// A student who starts with two minutes left gets two minutes, not a deadline
// in the past. The CHECK (deadline_at > started_at) would reject the row, so
// getting this wrong is a 500 on the "Bắt đầu" click rather than a bad timer.
func TestStartingAgainstTheCloseNeverYieldsADeadlineBehindTheStart(t *testing.T) {
	now := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)
	r := attempts.Rules{DurationMinutes: 45, ClosesAt: now.Add(2 * time.Minute)}

	if got := r.Deadline(now); !got.After(now) {
		t.Fatalf("deadline %v is not after the start %v", got, now)
	}
}

// [40-open-items.md P3] Once started, deadline_at wins: a close that passes
// mid-attempt does not truncate the paper. Deadline is called once, at
// creation, and this pins the reason it must not be recomputed on resume.
func TestTheDeadlineDoesNotMoveWhenTheAssignmentCloses(t *testing.T) {
	start := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)
	r := attempts.Rules{DurationMinutes: 60, ClosesAt: start.Add(90 * time.Minute)}
	atStart := r.Deadline(start)

	if atStart.Sub(start) != 60*time.Minute {
		t.Errorf("the attempt's own deadline is %v after its start, want 60m", atStart.Sub(start))
	}

	resumedAt := start.Add(2 * time.Hour)
	if recomputed := r.Deadline(resumedAt); !recomputed.Before(resumedAt) {
		t.Fatalf("recomputing at %v gave %v, which is not in the past; this test no longer proves anything",
			resumedAt, recomputed)
	}
}
