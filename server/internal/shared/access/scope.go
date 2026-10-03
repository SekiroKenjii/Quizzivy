package access

// Scope is whose rows a repository may read or write for a request: those the
// user owns or is given, or, with All, every teacher's.
type Scope struct {
	UserID string
	All    bool
}
