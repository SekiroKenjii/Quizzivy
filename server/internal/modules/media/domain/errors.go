package domain

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupportedType = errors.New("media: not an audio or image file we accept")
	ErrUnmeasurable    = errors.New("media: the audio's duration cannot be read")
	ErrTooLarge        = errors.New("media: file is larger than the limit")
	ErrTooLong         = errors.New("media: audio is longer than the limit")
)

// ErrImageTooLarge is ErrTooLarge for a file identified as an image and over
// the image limit, so the refusal can name that limit.
var ErrImageTooLarge = fmt.Errorf("%w: an image over the image limit", ErrTooLarge)

// ErrQuotaExceeded refuses a file that would take its owner's library past the quota.
var ErrQuotaExceeded = errors.New("media: the owner's library would exceed its quota")

// ErrPlayLimitOnImage refuses a default play limit on anything but audio.
var ErrPlayLimitOnImage = errors.New("media: only audio takes a play limit")

// ErrInvalidPlayLimit refuses a default play limit outside 0 to MaxDefaultPlays.
var ErrInvalidPlayLimit = errors.New("media: the play limit is out of range")

// ErrInvalidName refuses a display name that is blank once trimmed or longer than MaxDisplayNameLength.
var ErrInvalidName = errors.New("media: the display name is blank or too long")

// ErrNothingToUpdate refuses an update that names no change.
var ErrNothingToUpdate = errors.New("media: the update changes nothing")

// ErrReferenced rejects deletion while a published version or independent group still depends on the asset.
var ErrReferenced = errors.New("media: asset is referenced by assessment content")

var ErrNoID = errors.New("media: could not generate an asset id")

// ErrForbidden is a student asking for an asset they cannot reach. Deliberately
// indistinguishable from an asset that does not exist: telling the caller which
// one it was turns the endpoint into an oracle for which asset ids are real.
var ErrForbidden = errors.New("media: asset not reachable by this student")

var ErrNotFound = errors.New("media: asset not found")

// ErrKindMismatch refuses a replacement with another media kind.
var ErrKindMismatch = errors.New("media: replacement kind differs")

// ReplacementOutcome distinguishes confirmed noncommit from uncertain and confirmed commit.
type ReplacementOutcome uint8

const (
	ReplacementUnknown ReplacementOutcome = iota
	ReplacementNotCommitted
	ReplacementCommitted
)

// ReplacementError preserves transaction outcome and any failed rollback.
type ReplacementError struct {
	Outcome       ReplacementOutcome
	Cause         error
	RollbackError error
}

func (e *ReplacementError) Error() string {
	return fmt.Sprint("media: replacement: ", errors.Join(e.Cause, e.RollbackError))
}
func (e *ReplacementError) Unwrap() []error { return []error{e.Cause, e.RollbackError} }

// ReplacementCleanupError retains the original refusal and failed fresh-object compensation.
type ReplacementCleanupError struct {
	Cause   error
	Cleanup error
}

func (e *ReplacementCleanupError) Error() string {
	return fmt.Sprint("media: replacement cleanup: ", errors.Join(e.Cause, e.Cleanup))
}
func (e *ReplacementCleanupError) Unwrap() []error { return []error{e.Cause, e.Cleanup} }
