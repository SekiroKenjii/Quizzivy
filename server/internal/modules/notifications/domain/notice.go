package domain

import "unicode/utf8"

// MaxDedupeKey is the longest dedupe key a notice may carry, in characters.
const MaxDedupeKey = 200

// Merge says what a Notice does to the notification its user already holds
// under the same dedupe key. It is a closed set, and the store applies it
// inside the one statement that writes the row, so that no caller can make a
// merge depend on a row read beforehand. The zero value is not a mode.
type Merge int

const (
	// Replace stores the notice's params in place of the stored ones.
	Replace Merge = iota + 1
	// Add stores the notice's params with count and toGrade, where the notice
	// carries them, added to the stored ones; a stored row without one counts
	// as zero.
	Add
)

// Notice is one notification as a producer hands it in. DedupeKey names the
// thing it is about for its user: a second notice with the same user and key
// merges into the first and makes it unread again, instead of adding a row.
type Notice struct {
	UserID    string
	Kind      Kind
	Params    Params
	Target    *Target
	DedupeKey string
	Merge     Merge
}

// Validate answers the first of ErrNoRecipient, ErrUnknownKind,
// ErrParamsMismatch, ErrInvalidParams, ErrInvalidTarget, ErrInvalidDedupeKey
// and ErrUnknownMerge that applies, and nil for a notice the store can write.
func (n Notice) Validate() error {
	if n.UserID == "" {
		return ErrNoRecipient
	}
	if !n.Kind.Known() {
		return ErrUnknownKind
	}
	if n.Params == nil || n.Params.Kind() != n.Kind {
		return ErrParamsMismatch
	}
	if _, err := Encode(n.Params); err != nil {
		return err
	}
	if n.Target != nil {
		if err := n.Target.Validate(); err != nil {
			return err
		}
	}
	if length := utf8.RuneCountInString(n.DedupeKey); length < 1 || length > MaxDedupeKey {
		return ErrInvalidDedupeKey
	}
	if n.Merge != Replace && n.Merge != Add {
		return ErrUnknownMerge
	}
	return nil
}
