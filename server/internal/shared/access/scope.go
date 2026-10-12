package access

// Scope is whose rows a repository may read or write for a request: those the
// user owns or is given, or, with All, every teacher's.
type Scope struct {
	UserID string
	All    bool
}

// Own returns the scope narrowed to the user's own rows: the same user with
// All cleared. The teacher workspace's content lists pass it, so a caller who
// holds scope.all lists what any other teacher would, while a read or a write
// by id keeps the scope the request resolved.
func (s Scope) Own() Scope {
	s.All = false
	return s
}
