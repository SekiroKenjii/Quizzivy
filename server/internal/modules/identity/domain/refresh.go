package domain

import (
	"errors"
)

// ErrRefreshRejected covers every ordinary refresh failure: no cookie, an
// unknown token, an expired one, or a suspended account. A suspended account is
// not called out, for the same reason Login does not call it out.
var ErrRefreshRejected = errors.New("refresh rejected")

// ErrRefreshReused is §5.2 reuse detection firing: the presented token had
// already been rotated, and the whole family has now been revoked.
//
// Kept distinct from ErrRefreshRejected for the VICTIM's benefit, not the
// attacker's. By the time this is returned the family is dead, so the fact
// leaks no access; but "someone else used your session" is a thing a student
// can act on, and "your session expired" is not.
var ErrRefreshReused = errors.New("refresh token reused")
