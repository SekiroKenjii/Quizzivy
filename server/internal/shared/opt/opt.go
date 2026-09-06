package opt

// String is v as a nullable column value: nil for the empty string.
func String(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func Deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func Int(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
