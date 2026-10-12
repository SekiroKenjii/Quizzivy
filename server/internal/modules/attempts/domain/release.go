package domain

import "time"

// Release is when a student's result reaches them: as soon as they hand in,
// or once the assignment has closed for them.
type Release string

const (
	ReleaseOnSubmit   Release = "on_submit"
	ReleaseAfterClose Release = "after_close"
)

// ClassAverageFloor is the fewest students a class average may be drawn from.
// Below it the average of one or two is, to the others, a person's score.
const ClassAverageFloor = 3

// ReviewManager holds the rules for what a result may show, and when.
type ReviewManager struct{}

// Withheld reports whether a result is still held back: its assignment
// releases after close and the reader's close has not come. A result released
// on submit is never withheld.
func (ReviewManager) Withheld(release Release, now, close time.Time) bool {
	return release == ReleaseAfterClose && now.Before(close)
}

// Effective is the policy a student may read at this moment: the stored flags
// while the result is released, and none of them while it is withheld. The
// release and the class-average switch are the teacher's choices and stay.
func (ReviewManager) Effective(stored ReviewPolicy, withheld bool) ReviewPolicy {
	if withheld {
		stored.ShowScore, stored.ShowCorrectAnswers, stored.ShowExplanations = false, false, false
	}
	return stored
}

// ShowsAverage reports whether a result carries the class average: the switch
// is on, this result is released, the assignment has closed, and at least
// ClassAverageFloor students qualify.
func (ReviewManager) ShowsAverage(stored ReviewPolicy, withheld, closed bool, qualifying int) bool {
	return stored.ShowClassAverage && !withheld && closed && qualifying >= ClassAverageFloor
}
