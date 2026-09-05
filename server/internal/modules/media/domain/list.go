package domain

// ListInput selects a page of the library.
type ListInput struct {
	Kind  *Kind
	Page  int // 1-based; below 1 reads as the first
	Limit int // clamped to [1, MaxLimit]
}
