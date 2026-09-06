package domain

import (
	"errors"
)

var (
	ErrNotFound        = errors.New("tests: not found")
	ErrStaleWrite      = errors.New("tests: edited elsewhere since the version read")
	ErrUnknownQuestion = errors.New("tests: outline references a question that does not exist")
)

// ErrNotPublished is returned when a test has no version to render.
var ErrNotPublished = errors.New("tests: no published version")

var (
	ErrDraftNotFound = errors.New("publish: test not found")
	// ErrNoContent is a test with no sections at all.
	ErrNoContent = errors.New("publish: test has no sections")
)
