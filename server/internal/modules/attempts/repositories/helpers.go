package repositories

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// optionalIP is optional for the inet column, which rejects an empty string
// where a text column would store it.
func optionalIP(s string) *string { return optional(s) }
