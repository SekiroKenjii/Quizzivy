package domain

import (
	"errors"
)

var (
	ErrNotFound = errors.New("questions: not found")
	// ErrReferenced is a delete refused because a draft outline still uses it.
	ErrReferenced = errors.New("questions: referenced by a draft test outline")
)

// ErrMediaNotFound is a well-formed asset id that resolves to nothing.
var ErrMediaNotFound = errors.New("questions: media asset not found")

// ProseMediaError refuses a prompt or an explanation that shows media the
// writer may not use: a missing, deleted or unreadable asset alike (D-064
// ruling 5), or one whose kind differs from its block's. It is an
// ErrMediaNotFound, so a group's write answers it as a missing file.
type ProseMediaError struct {
	Field     string
	WrongKind bool
}

func (e *ProseMediaError) Error() string {
	if e.WrongKind {
		return "questions: " + e.Field + " shows media of another kind"
	}
	return "questions: " + e.Field + " shows a media asset that was not found"
}

func (e *ProseMediaError) Unwrap() error { return ErrMediaNotFound }
