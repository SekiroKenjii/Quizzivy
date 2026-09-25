package domain

import (
	"regexp"
	"strconv"
	"strings"
)

var pointPattern = regexp.MustCompile(`^[0-9]{1,6}(\.[0-9]{1,2})?$`)

// PointUnits returns exact hundredths for a positive numeric(8,2) question score.
func PointUnits(value string) (int64, bool) {
	if !pointPattern.MatchString(value) {
		return 0, false
	}
	whole, fraction, _ := strings.Cut(value, ".")
	integer, _ := strconv.ParseInt(whole, 10, 64)
	decimal, _ := strconv.ParseInt((fraction + "00")[:2], 10, 64)
	units := integer*100 + decimal
	return units, units > 0 && units <= 99_999_999
}
