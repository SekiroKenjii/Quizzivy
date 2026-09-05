package domain

import "time"

// CodeState is what decides whether a join code can still be redeemed.
type CodeState struct {
	SelfJoinEnabled bool
	RevokedAt       *time.Time
	ExpiresAt       time.Time
	MaxUses         *int
	UsesCount       int
}

// Usable reports PreviewOK or the reason the code is refused. A class with
// self-join closed answers exactly as a nonexistent code does, so the endpoints
// cannot be used to discover which classes exist.
func (c CodeState) Usable(now time.Time) PreviewOutcome {
	switch {
	case !c.SelfJoinEnabled:
		return PreviewInvalid
	case c.RevokedAt != nil:
		return PreviewRevoked
	case !c.ExpiresAt.After(now):
		return PreviewExpired
	case c.MaxUses != nil && c.UsesCount >= *c.MaxUses:
		return PreviewExhausted
	}
	return PreviewOK
}
