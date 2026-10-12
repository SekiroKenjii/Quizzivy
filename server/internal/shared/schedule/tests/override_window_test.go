package schedule_test

import (
	"quizzivy/internal/shared/schedule"
	"strings"
	"testing"
	"time"
)

func TestAnOverrideLiftsAnEarlyCloseOnlyWhenItReachesPastIt(t *testing.T) {
	early := closes.Add(-48 * time.Hour)
	w := schedule.Window{OpensAt: opens, ClosesAt: closes, ClosedAt: &early, DurationMin: 45, MaxAttempts: 1}
	if got := w.Close(); !got.Equal(early) {
		t.Fatalf("Close = %v, want the early close %v", got, early)
	}

	later := early.Add(time.Hour)
	lifted := w.WithOverride(&schedule.Override{ClosesAt: &later})
	if lifted.ClosedAt != nil || !lifted.ClosesAt.Equal(later) || !lifted.Close().Equal(later) {
		t.Errorf("an override past the early close gave %+v, want it to close at %v with no early close", lifted, later)
	}

	sooner := early.Add(-time.Hour)
	if got := w.WithOverride(&schedule.Override{ClosesAt: &sooner}); got != w {
		t.Errorf("an override before the early close gave %+v, want the window unchanged %+v", got, w)
	}
	if w.ClosedAt == nil || !w.ClosedAt.Equal(early) {
		t.Errorf("WithOverride changed its receiver: %+v", w)
	}
}

func TestAnOverrideBeyondTheWindowEndStillBeatsAnEarlyCloseAfterIt(t *testing.T) {
	earlyAfterEnd := closes.Add(3 * time.Hour)
	w := schedule.Window{OpensAt: opens, ClosesAt: closes, ClosedAt: &earlyAfterEnd}
	later := closes.Add(time.Hour)
	if got := w.WithOverride(&schedule.Override{ClosesAt: &later}); !got.Close().Equal(later) || got.ClosedAt != nil {
		t.Errorf("WithOverride = %+v, want the close at %v", got, later)
	}
}

func TestAStudentWithoutAnOverrideRowReadsNone(t *testing.T) {
	if got := (schedule.OverrideColumns{}).Override(); got != nil {
		t.Errorf("columns of a LEFT JOIN that found nothing gave %+v, want nil", got)
	}
	extra := 0
	minutes := 90
	later := closes.Add(time.Hour)
	got := schedule.OverrideColumns{ClosesAt: &later, DurationMin: &minutes, ExtraAttempts: &extra}.Override()
	if got == nil || got.ClosesAt == nil || !got.ClosesAt.Equal(later) || got.DurationMin == nil || *got.DurationMin != 90 || got.ExtraAttempts != 0 {
		t.Errorf("columns of a row gave %+v", got)
	}
}

func TestTheOverrideJoinNamesTheStudentItIsGiven(t *testing.T) {
	join := schedule.OverrideJoin("$1::uuid")
	for _, want := range []string{"LEFT JOIN app.assignment_student_overrides o", "o.assignment_id = a.id", "o.student_id = $1::uuid"} {
		if !strings.Contains(join, want) {
			t.Errorf("OverrideJoin = %q, want it to contain %q", join, want)
		}
	}
	for _, column := range []string{"o.closes_at", "o.duration_minutes", "o.extra_attempts"} {
		if !strings.Contains(schedule.OverrideSelect, column) {
			t.Errorf("OverrideSelect = %q, want it to name %s", schedule.OverrideSelect, column)
		}
	}
}
