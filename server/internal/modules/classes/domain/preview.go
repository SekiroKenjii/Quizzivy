package domain

import (
	"errors"
)

// PreviewOutcome is why a code was or was not accepted. Each maps to a distinct
// error code so the student gets an accurate message -- "ask your teacher for a
// new code" is useful where "wrong code" is not -- without any of them naming a
// class (§6.5).
type PreviewOutcome int

const (
	PreviewOK PreviewOutcome = iota
	PreviewInvalid
	PreviewRevoked
	PreviewExpired
	PreviewExhausted
)

// PreviewResult carries exactly the three fields §6.5 permits, and the outcome.
type PreviewResult struct {
	Outcome     PreviewOutcome
	ClassID     string
	ClassName   string
	TeacherName string
}

// ErrNoTeacher means the install has no active admin, so no class can name one.
// An operational fault, not a bad request.
var ErrNoTeacher = errors.New("join: no active teacher account")

// CodeRow is everything a preview decision needs, in one round trip.
type CodeRow struct {
	CodeState
	ClassID     string
	ClassName   string
	CodeHash    []byte
	TeacherName *string
}
