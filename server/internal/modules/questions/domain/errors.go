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
