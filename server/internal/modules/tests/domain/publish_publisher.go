package domain

// PublishRequest is one publish.
type PublishRequest struct {
	TestID    string
	ActorID   string
	IP        string
	UserAgent string
}

// Totals sums the points the version is scored out of, frozen so the
// denominator on an old attempt cannot drift.
func Totals(d DraftContent) (string, int) {
	var total float64
	count := 0
	for _, section := range d.Sections {
		for _, q := range section.Questions {
			total += parsePoints(q.Points)
			count++
		}
	}
	return formatPoints(total), count
}
