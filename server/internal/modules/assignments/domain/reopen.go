package domain

import (
	"errors"
)

var (
	ErrNotClosed    = errors.New("assignments: not closed")
	ErrBlankReason  = errors.New("assignments: reason is blank")
	ErrClosesInPast = errors.New("assignments: closes_at is not ahead")
)
