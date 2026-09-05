package domain

import (
	"errors"
)

var (
	ErrLastLoginMethod           = errors.New("google is the account's only login method")
	ErrEmailBelongsToAnotherUser = errors.New("that Google address belongs to another account")
)
