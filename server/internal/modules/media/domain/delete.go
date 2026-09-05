package domain

import (
	"errors"
	"time"
)

// ErrReferenced is a delete refused because a published version still uses the
// asset (§8, §15). Answered 409, not 403: the caller has every right to the
// asset, the asset is simply not deletable while something depends on it.
var ErrReferenced = errors.New("media: asset is referenced by a published version")

// ReferencedError is ErrReferenced carrying the versions that block the
// delete, so the refusal can name them (A-07). errors.Is(err, ErrReferenced)
// still holds.
type ReferencedError struct{ Tests []TestRef }

func (e *ReferencedError) Error() string { return ErrReferenced.Error() }

func (e *ReferencedError) Is(target error) bool { return target == ErrReferenced }

// DeleteInput is one soft delete, with the audit context it must record.
type DeleteInput struct {
	ID        string
	ActorID   string
	Now       time.Time
	IP        string
	UserAgent string
}
