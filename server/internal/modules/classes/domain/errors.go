package domain

import (
	"errors"
)

var ErrNotFound = errors.New("classes: not found")

var ErrNotAStudent = errors.New("classes: not a student")

// ErrEmailTaken is a signup racing another signup for the same address. The
// §5.3 resolution order makes this nearly unreachable -- an existing email is
// matched and linked one branch earlier -- so it means two requests arrived
// inside the same microseconds, and the caller should simply try again.
var ErrEmailTaken = errors.New("join: email already registered")

var ErrClassNotFound = errors.New("join: class not found")

// ErrNoTeacher means the install has no active admin, so no class can name one.
// An operational fault, not a bad request.
var ErrNoTeacher = errors.New("join: no active teacher account")
