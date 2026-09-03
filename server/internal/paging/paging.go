// Package paging is O-20's arithmetic, in one place so every list clamps the
// same way and reports the same three numbers.
package paging

// Page is what a list returns beside its rows: the page it actually served,
// the size it actually used, and how many rows match the caller's filters --
// present even on an empty page past the end, because that is the number a
// page count is drawn from.
type Page struct {
	Number int
	Size   int
	Total  int
}

// Clamp turns a caller's page and limit into what the query will use. A page
// below 1 is the first; a limit of 0 is the resource's default and one above
// max is max. The offset is what OFFSET gets.
func Clamp(page, limit, defaultLimit, maxLimit int) (number, size, offset int) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return page, limit, (page - 1) * limit
}
