package jobs

import (
	"context"
	"time"
)

// ImportRunners is the worker's queues in the order it drains them: RunOne
// serves the first queue that has work, so an earlier queue is served
// whenever it has any, and NextDue reports the earliest the queues fall due.
type ImportRunners []ImportRunner

// RunOne runs one job of the first queue that has one, and reports whether
// any did. It stops at the first error.
func (c ImportRunners) RunOne(ctx context.Context) (bool, error) {
	for _, runner := range c {
		worked, err := runner.RunOne(ctx)
		if worked || err != nil {
			return worked, err
		}
	}
	return false, nil
}

// NextDue reports the earliest due time of any queue, and false when none has
// work waiting.
func (c ImportRunners) NextDue(ctx context.Context) (time.Time, bool, error) {
	var earliest time.Time
	pending := false
	for _, runner := range c {
		due, ok, err := runner.NextDue(ctx)
		if err != nil {
			return time.Time{}, false, err
		}
		if ok && (!pending || due.Before(earliest)) {
			earliest, pending = due, true
		}
	}
	return earliest, pending, nil
}
