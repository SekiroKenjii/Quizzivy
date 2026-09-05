package domain

import (
	"errors"
)

// ErrInvalidCredentials is returned for every failed login, whatever the actual
// cause: no such email, a Google-only account, a disabled account, or the wrong
// password.
var ErrInvalidCredentials = errors.New("invalid credentials")
