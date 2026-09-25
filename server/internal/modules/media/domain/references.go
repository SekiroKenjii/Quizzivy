package domain

// TestRef names one published version that uses an asset.
type TestRef struct {
	ID      string
	Title   string
	Version int
}

// ReferencedError is ErrReferenced carrying the versions and independent groups that prevent deletion.
type ReferencedError struct {
	Tests  []TestRef
	Groups []GroupRef
}

// GroupRef names an independent group that owns a material or member reference to an asset.
type GroupRef struct {
	ID     string
	Title  string
	TestID *string
}

func (e *ReferencedError) Error() string { return ErrReferenced.Error() }

func (e *ReferencedError) Is(target error) bool { return target == ErrReferenced }
