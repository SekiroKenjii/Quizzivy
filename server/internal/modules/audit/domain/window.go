package domain

import (
	"time"
)

// Range is how far back a reader asked to look.
type Range string

// The ranges the list offers, and the extra one only an export has.
const (
	Today   Range = "today"
	Week    Range = "7d"
	Month   Range = "30d"
	Forever Range = "all"
	Custom  Range = "custom"
	Year    Range = "year"
)

// DefaultZone is the calendar zone a day is counted in when the viewer has
// stored none that this server knows.
const DefaultZone = "Asia/Ho_Chi_Minh"

const dateLayout = "2006-01-02"

// Window is the stretch of time a reader looks at: entries from From, which is
// included, up to To, which is not. A nil end is no bound.
type Window struct {
	From *time.Time
	To   *time.Time
}

// LocationOf loads zone, or the fixed offset of DefaultZone when it names no
// zone this server has.
func LocationOf(zone string) *time.Location {
	if zone != "" && zone != "Local" {
		if loc, err := time.LoadLocation(zone); err == nil {
			return loc
		}
	}
	if loc, err := time.LoadLocation(DefaultZone); err == nil {
		return loc
	}
	return time.FixedZone(DefaultZone, 7*60*60)
}

// ResolveWindow turns what a reader asked for into the instants it stands for,
// every day counted as the calendar day in loc as of now. Today starts at this
// day's midnight, Week and Month at the midnight of the sixth and the
// twenty-ninth day before it, so the day itself is among the seven or the
// thirty, and Year at the first of January. Custom reads from and to as
// YYYY-MM-DD, both days included. The empty range is Week. A from or to
// outside Custom, a Custom without both, a to before the from and a date that
// is not a date are a *RangeError; a range not named above is ErrUnknownRange.
func ResolveWindow(kind Range, from, to string, loc *time.Location, now time.Time) (Window, error) {
	if kind == "" {
		kind = Week
	}
	if kind != Custom {
		if field := firstGiven(from, to); field != "" {
			return Window{}, &RangeError{Field: field, Problem: RangeNotCustom}
		}
	}
	year, month, day := now.In(loc).Date()
	startOf := func(daysBack int) Window {
		start := time.Date(year, month, day-daysBack, 0, 0, 0, 0, loc)
		return Window{From: &start}
	}
	switch kind {
	case Today:
		return startOf(0), nil
	case Week:
		return startOf(6), nil
	case Month:
		return startOf(29), nil
	case Forever:
		return Window{}, nil
	case Year:
		start := time.Date(year, time.January, 1, 0, 0, 0, 0, loc)
		return Window{From: &start}, nil
	case Custom:
		return customWindow(from, to, loc)
	}
	return Window{}, ErrUnknownRange
}

func firstGiven(from, to string) string {
	switch {
	case from != "":
		return "from"
	case to != "":
		return "to"
	}
	return ""
}

func customWindow(from, to string, loc *time.Location) (Window, error) {
	switch {
	case from == "":
		return Window{}, &RangeError{Field: "from", Problem: RangeIncomplete}
	case to == "":
		return Window{}, &RangeError{Field: "to", Problem: RangeIncomplete}
	}
	first, err := time.ParseInLocation(dateLayout, from, loc)
	if err != nil {
		return Window{}, &RangeError{Field: "from", Problem: RangeMalformed}
	}
	last, err := time.ParseInLocation(dateLayout, to, loc)
	if err != nil {
		return Window{}, &RangeError{Field: "to", Problem: RangeMalformed}
	}
	if last.Before(first) {
		return Window{}, &RangeError{Field: "to", Problem: RangeReversed}
	}
	end := time.Date(last.Year(), last.Month(), last.Day()+1, 0, 0, 0, 0, loc)
	return Window{From: &first, To: &end}, nil
}
