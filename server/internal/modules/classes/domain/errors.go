package domain

import (
	"errors"
)

var ErrNotFound = errors.New("classes: not found")

// ErrNotAStudent means the user is not an active student the actor reaches:
// missing, disabled, staff or another teacher's student all answer alike, so a
// body id reveals nothing about accounts beyond the actor's reach.
var ErrNotAStudent = errors.New("classes: not a student")

// ErrEmailTaken is a signup racing another signup for the same address. The
// §5.3 resolution order makes this nearly unreachable -- an existing email is
// matched and linked one branch earlier -- so it means two requests arrived
// inside the same microseconds, and the caller should simply try again.
var ErrEmailTaken = errors.New("join: email already registered")

var ErrClassNotFound = errors.New("join: class not found")

// ErrNoActiveCode is a class the caller reaches that has no active join code.
var ErrNoActiveCode = errors.New("join: no active code")

// ErrNoTeacher means a class names no teacher, which classes.teacher_id's
// constraint rules out. An operational fault, not a bad request.
var ErrNoTeacher = errors.New("join: no active teacher account")

var (
	ErrReferenced  = errors.New("class: retained history or assignments reference this resource")
	ErrNotArchived = errors.New("class: deactivate or close before deleting")
)
