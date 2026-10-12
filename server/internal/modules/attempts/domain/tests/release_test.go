package domain_test

import (
	"quizzivy/internal/modules/attempts/domain"
	"testing"
	"time"
)

func TestAResultIsWithheldOnlyBeforeTheCloseOfAnAfterCloseAssignment(t *testing.T) {
	closes := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

	for _, c := range []struct {
		name    string
		release domain.Release
		now     time.Time
		want    bool
	}{
		{"on submit, long before the close", domain.ReleaseOnSubmit, closes.Add(-24 * time.Hour), false},
		{"on submit, after the close", domain.ReleaseOnSubmit, closes.Add(time.Hour), false},
		{"after close, long before the close", domain.ReleaseAfterClose, closes.Add(-24 * time.Hour), true},
		{"after close, a nanosecond before the close", domain.ReleaseAfterClose, closes.Add(-time.Nanosecond), true},
		{"after close, the instant of the close", domain.ReleaseAfterClose, closes, false},
		{"after close, after the close", domain.ReleaseAfterClose, closes.Add(time.Hour), false},
	} {
		if got := domain.Reviews.Withheld(c.release, c.now, closes); got != c.want {
			t.Errorf("%s: withheld=%v, want %v", c.name, got, c.want)
		}
	}
}

func TestWithholdingTakesTheThreeFlagsAndKeepsTheTeachersChoices(t *testing.T) {
	stored := domain.ReviewPolicy{
		ShowScore: true, ShowCorrectAnswers: true, ShowExplanations: true,
		Release: domain.ReleaseAfterClose, ShowClassAverage: true,
	}

	if got := domain.Reviews.Effective(stored, false); got != stored {
		t.Errorf("a released result changed the policy: %+v", got)
	}
	got := domain.Reviews.Effective(stored, true)
	if got.ShowScore || got.ShowCorrectAnswers || got.ShowExplanations {
		t.Errorf("a withheld result still shows: %+v", got)
	}
	if got.Release != domain.ReleaseAfterClose || !got.ShowClassAverage {
		t.Errorf("withholding changed the release or the average switch: %+v", got)
	}
	if !stored.ShowScore {
		t.Error("withholding rewrote the stored policy")
	}
}

func TestTheClassAverageNeedsThreeStudentsAnAnsweredSwitchAndAClosedWindow(t *testing.T) {
	on := domain.ReviewPolicy{ShowClassAverage: true}

	for qualifying := 0; qualifying <= 5; qualifying++ {
		want := qualifying >= domain.ClassAverageFloor
		if got := domain.Reviews.ShowsAverage(on, false, true, qualifying); got != want {
			t.Errorf("%d qualifying students: shown=%v, want %v", qualifying, got, want)
		}
	}
	if domain.ClassAverageFloor != 3 {
		t.Errorf("the floor is %d, want 3", domain.ClassAverageFloor)
	}
	if domain.Reviews.ShowsAverage(domain.ReviewPolicy{}, false, true, 30) {
		t.Error("the average is shown with the switch off")
	}
	if domain.Reviews.ShowsAverage(on, false, false, 30) {
		t.Error("the average is shown before the assignment has closed")
	}
	if domain.Reviews.ShowsAverage(on, true, true, 30) {
		t.Error("the average is shown on a withheld result")
	}
}
