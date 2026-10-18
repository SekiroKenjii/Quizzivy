package domain

import (
	"errors"
	"fmt"
)

// Impact is what a sign-in policy would do to the active accounts: how many
// would be left with no method that counts, how many of those hold the
// built-in Admin role, how many active built-in Admins there are, and whether
// the user asking is among those left without one.
type Impact struct {
	AffectedUsers  int
	AffectedAdmins int
	ActiveAdmins   int
	ActorLocked    bool
}

// Lockout reports whether the policy would leave the user asking, or every
// active built-in Admin, with no way to sign in. Such a policy is never
// stored.
func (i Impact) Lockout() bool {
	return i.ActorLocked || i.AffectedAdmins >= i.ActiveAdmins
}

var (
	// ErrStale is a save of a document read at a revision that is no longer the stored one.
	ErrStale = errors.New("organization: the settings were saved elsewhere since they were read")
	// ErrLockout is a save refused because of what its sign-in policy would do. LockedOut carries the counts.
	ErrLockout = errors.New("organization: the sign-in policy would lock the Admins out")
	// ErrNoLogo is a request for a logo the centre does not have.
	ErrNoLogo = errors.New("organization: no logo of that variant")
	// ErrStorageOff is an upload, or a read of a logo's bytes, on a server with no object storage.
	ErrStorageOff = errors.New("organization: object storage is not configured")
	// ErrLogoTooLarge is a file over MaxPNGBytes.
	ErrLogoTooLarge = errors.New("organization: the logo is over its size limit")
	// ErrLogoUnsupported is an upload that is not a PNG, an SVG included.
	ErrLogoUnsupported = errors.New("organization: the logo is not a png")
	// ErrLogoUnreadable is a PNG whose header or pixels cannot be read.
	ErrLogoUnreadable = errors.New("organization: the logo cannot be read")
	// ErrLogoDimensions is a logo that is not square within 2%, has a side outside MinLogoSide to MaxLogoSide, or needs more memory to decode than the server allows.
	ErrLogoDimensions = errors.New("organization: the logo's dimensions are not allowed")
)

// LockedOut is the refusal of a save whose policy would lock the Admins out,
// with the impact it was judged on. It satisfies errors.Is(err, ErrLockout).
type LockedOut struct {
	Impact Impact
}

func (e *LockedOut) Error() string {
	return fmt.Sprintf("%s: %d accounts, %d admins", ErrLockout, e.Impact.AffectedUsers, e.Impact.AffectedAdmins)
}

func (e *LockedOut) Is(target error) bool { return target == ErrLockout }

// The limits an uploaded logo keeps.
const (
	MaxPNGBytes = 1 << 20
	MinLogoSide = 256
	MaxLogoSide = 2048
)

// LogoFile is a checked logo: its bytes as they are stored, its type and its size in pixels.
type LogoFile struct {
	Bytes  []byte
	Type   LogoType
	Width  int
	Height int
}

// RetentionImpact is what the retention sweep would do today under a
// retention policy, counted with the sweep's own rules. Answers is nil when
// the policy keeps answers for ever, Accounts when it never anonymises.
type RetentionImpact struct {
	Answers     *AnswersImpact
	FocusEvents int64
	Accounts    *AccountsImpact
}

// AnswersImpact is the ended attempts that hold an answer to remove, and
// those answers.
type AnswersImpact struct {
	Attempts int64
	Answers  int64
}

// AccountsImpact is the disabled accounts that would be anonymised, and those
// left because they own classes or content.
type AccountsImpact struct {
	Anonymise     int64
	SkippedOwners int64
}
