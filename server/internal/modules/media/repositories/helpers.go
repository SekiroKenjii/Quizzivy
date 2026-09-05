package repositories

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
