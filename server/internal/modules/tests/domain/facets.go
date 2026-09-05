package domain

// StatusFacets is how many tests each status holds for one search.
type StatusFacets struct {
	All       int
	Draft     int
	Published int
	Archived  int
}
