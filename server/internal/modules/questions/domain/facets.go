package domain

// TypeFacets is how many bank questions each type holds for one search.
type TypeFacets struct {
	All    int
	ByType map[Type]int
}
