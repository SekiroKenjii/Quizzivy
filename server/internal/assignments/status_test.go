package assignments_test

import (
	"testing"
	"time"

	"quizzivy/internal/assignments"
)

// D-18: status is a pure function of the window and an optional early close.
// It is the reason there is no scheduler and no column that can go stale, so it
// is worth pinning at the boundaries rather than in the middle.
func TestStatusIsDerivedFromTheWindow(t *testing.T) {
	opens := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)
	closes := time.Date(2026, 8, 29, 17, 0, 0, 0, time.UTC)
	early := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name     string
		now      time.Time
		closedAt *time.Time
		want     assignments.Status
	}{
		{"before it opens", opens.Add(-time.Second), nil, assignments.Scheduled},
		{"exactly at opens_at is open", opens, nil, assignments.Open},
		{"inside the window", opens.Add(time.Hour), nil, assignments.Open},
		{"exactly at closes_at is closed", closes, nil, assignments.Closed},
		{"after it closes", closes.Add(time.Second), nil, assignments.Closed},
		{"closed early wins over an open window", early.Add(time.Minute), &early, assignments.Closed},
		{"a future early-close has not happened yet", opens.Add(time.Hour), &closes, assignments.Open},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := assignments.StatusAt(tc.now, opens, closes, tc.closedAt)
			if got != tc.want {
				t.Errorf("want %s, got %s", tc.want, got)
			}
		})
	}
}
