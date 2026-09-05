package domain

import (
	"errors"
)

// ErrMediaNotFound is a well-formed asset id that resolves to nothing.
var ErrMediaNotFound = errors.New("questions: media asset not found")

type WriteRequest struct {
	ID        string // empty to create
	Input     Input
	ActorID   string
	IP        string
	UserAgent string
}
