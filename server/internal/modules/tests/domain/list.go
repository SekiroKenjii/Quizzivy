package domain

// ListInput selects a page of tests.
type ListInput struct {
	Status *Status
	// Tags filter by the tags of the questions a test CONTAINS.
	Tags  []string
	Query string
	Page  int
	Limit int
}
