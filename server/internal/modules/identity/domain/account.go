package domain

import (
	"errors"
)

var (
	ErrAccountDisabled  = errors.New("account disabled")
	ErrNoPasswordSet    = errors.New("account has no password")
	ErrPasswordTooShort = errors.New("new password is too short")
	ErrPasswordTooLong  = errors.New("new password is too long")
)

// Password bounds from api/openapi.yaml. The maximum exists because Argon2id
// hashes whatever it is given, and a megabyte of "password" is a free way to
// burn CPU on an authenticated endpoint.
const (
	MinPasswordLength = 8
	MaxPasswordLength = 512
)
