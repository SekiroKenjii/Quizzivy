package domain

import (
	"context"
	"time"
)

// AvatarPrefix is the start of every profile photo's key: a photo of a user is
// stored under AvatarPrefix + their id + "/".
const AvatarPrefix = "avatars/"

// AvatarSweeps records the anonymised accounts whose profile photo objects the
// housekeeping has removed (D-050). Anonymisation clears the photo key but
// cannot reach the object store, so the objects stay until the sweep empties
// the account's prefix and marks it.
type AvatarSweeps interface {
	// Due returns the ids of at most limit anonymised accounts that are not
	// marked yet, in id order. An account is anonymised when it holds exactly
	// what anonymisation writes: the address 'anonymous-' || id ||
	// '@anonymous.invalid', a disabled_at and no photo key.
	Due(ctx context.Context, limit int) ([]string, error)
	// Mark records that the account's photo objects were removed at the given
	// moment. Marking an account a second time keeps the first mark.
	Mark(ctx context.Context, userID string, at time.Time) error
}
