package domain

import (
	"errors"
	"strings"
)

// Request is who is intervening and from where, for the audit AttemptRecord every
// intervention writes (§13.4).
type Request struct {
	ActorID   string
	IP        string
	UserAgent string
}

var (
	ErrBlankReason   = errors.New("attempts: reason is blank")
	ErrAttemptVoided = errors.New("attempts: attempt is voided")
)

// CleanReason rejects a reason that is only whitespace: the schema's minLength
// cannot tell "   " from a reason, and a blank one defeats the audit AttemptRecord.
func CleanReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", ErrBlankReason
	}
	return reason, nil
}
