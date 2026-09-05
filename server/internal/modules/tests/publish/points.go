package publish

import "strconv"

// Points cross package boundaries as decimal strings, because the column is
// numeric(8,2) and a binary float cannot represent every value it holds.
// Summing goes through float64, which is exact for the two-decimal values in
// range, and the result is formatted straight back to two places.
func parsePoints(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func formatPoints(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
