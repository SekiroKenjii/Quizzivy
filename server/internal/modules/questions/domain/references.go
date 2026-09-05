package domain

// TestRef names a draft test whose outline holds the question.
type TestRef struct {
	ID    string
	Title string
}

// ReferencedError is ErrReferenced carrying the drafts that block the delete,
// so the refusal can say where to look (A-06a). errors.Is(err, ErrReferenced)
// still holds.
type ReferencedError struct{ Tests []TestRef }

func (e *ReferencedError) Error() string { return ErrReferenced.Error() }

func (e *ReferencedError) Is(target error) bool { return target == ErrReferenced }
