package domain

// ListInput selects a page of the bank.
//
// Types and Tags are each OR-ed within themselves and AND-ed with each other,
// which is what A-06's rail of checkboxes and chips means: ticking a second
// type widens the results, adding a tag from the other group narrows them.
type ListInput struct {
	Types    []Type
	Tags     []string
	HasAudio *bool
	Query    string
	Page     int
	Limit    int
}
