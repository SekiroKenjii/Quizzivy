package domain

import (
	"errors"
	"fmt"
)

// ErrExportTooLarge is what an export of more than MaxExportRows entries is
// refused with.
var ErrExportTooLarge = errors.New("audit: the export holds more than MaxExportRows entries")

// ErrUnknownCategory is what a category outside the six is refused with.
var ErrUnknownCategory = errors.New("audit: unknown category")

// ErrUnknownRange is what a range or period the log does not offer is refused
// with.
var ErrUnknownRange = errors.New("audit: unknown range")

// RangeProblem says what is wrong with the dates a caller sent.
type RangeProblem string

// The ways a custom range can be wrong.
const (
	// RangeNotCustom: from or to was sent without the custom range.
	RangeNotCustom RangeProblem = "not_custom"
	// RangeIncomplete: the custom range lacks its from or its to.
	RangeIncomplete RangeProblem = "incomplete"
	// RangeReversed: to is a day before from.
	RangeReversed RangeProblem = "reversed"
	// RangeMalformed: a date is not YYYY-MM-DD.
	RangeMalformed RangeProblem = "malformed"
)

// RangeError names the parameter a refused range is about, so the transport can
// answer on the field.
type RangeError struct {
	Field   string
	Problem RangeProblem
}

func (e *RangeError) Error() string {
	return fmt.Sprintf("audit: range: %s: %s", e.Field, e.Problem)
}
