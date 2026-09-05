package domain

// TestRef names one published version that uses an asset.
type TestRef struct {
	ID      string
	Title   string
	Version int
}

// ReferencedError is ErrReferenced carrying the versions that block the
// delete, so the refusal can name them (A-07). errors.Is(err, ErrReferenced)
// still holds.
type ReferencedError struct{ Tests []TestRef }

func (e *ReferencedError) Error() string { return ErrReferenced.Error() }

func (e *ReferencedError) Is(target error) bool { return target == ErrReferenced }
