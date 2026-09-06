package domain

import (
	"errors"
)

var (
	ErrUnsupportedType = errors.New("media: not an audio or image file we accept")
	ErrUnmeasurable    = errors.New("media: the audio's duration cannot be read")
	ErrTooLarge        = errors.New("media: file is larger than the limit")
	ErrTooLong         = errors.New("media: audio is longer than the limit")
)

// ErrReferenced is a delete refused because a published version still uses the
// asset (§8, §15). Answered 409, not 403: the caller has every right to the
// asset, the asset is simply not deletable while something depends on it.
var ErrReferenced = errors.New("media: asset is referenced by a published version")

var ErrNoID = errors.New("media: could not generate an asset id")

// ErrForbidden is a student asking for an asset they cannot reach. Deliberately
// indistinguishable from an asset that does not exist: telling the caller which
// one it was turns the endpoint into an oracle for which asset ids are real.
var ErrForbidden = errors.New("media: asset not reachable by this student")

var ErrNotFound = errors.New("media: asset not found")
