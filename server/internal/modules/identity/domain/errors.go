package domain

import (
	"errors"
)

var ErrUserNotFound = errors.New("user not found")

// ErrInvalidCredentials is returned for every failed login, whatever the actual
// cause: no such email, a Google-only account, a disabled account, or the wrong
// password.
var ErrInvalidCredentials = errors.New("invalid credentials")

var (
	ErrAccountDisabled  = errors.New("account disabled")
	ErrNoPasswordSet    = errors.New("account has no password")
	ErrPasswordTooShort = errors.New("new password is too short")
	ErrPasswordTooLong  = errors.New("new password is too long")
)

// ErrRefreshRejected covers every ordinary refresh failure: no cookie, an
// unknown token, an expired one, or a suspended account. A suspended account is
// not called out, for the same reason Login does not call it out.
var ErrRefreshRejected = errors.New("refresh rejected")

// ErrRefreshReused is §5.2 reuse detection firing: the presented token had
// already been rotated, and the whole family has now been revoked.
var ErrRefreshReused = errors.New("refresh token reused")

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

var (
	ErrLastLoginMethod           = errors.New("google is the account's only login method")
	ErrEmailBelongsToAnotherUser = errors.New("that Google address belongs to another account")
)

// ErrIdentityAlreadyLinked means the account already has an identity from this
// provider, and it is a different one. D-08's UNIQUE (user_id, provider) is
// what makes that detectable rather than silently creating a second link.
var ErrIdentityAlreadyLinked = errors.New("identity already linked")

var (
	ErrGoogleExchangeFailed     = errors.New("google: code exchange failed")
	ErrGoogleRedirectNotAllowed = errors.New("google: redirect uri is not allowed")
	ErrGoogleTokenInvalid       = errors.New("google: id token is invalid")
	ErrGoogleEmailUnverified    = errors.New("google: email is not verified")
	ErrAccountNotProvisioned    = errors.New("account not provisioned")
	ErrGoogleUnavailable        = errors.New("google sign-in is not configured")
	ErrSelfEnrolNotAvailable    = errors.New("join-code signup is not implemented yet")
)

var (
	ErrInvalidHash        = errors.New("password hash is not in PHC format")
	ErrUnsupportedVariant = errors.New("password hash is not argon2id")
	ErrIncompatibleAlg    = errors.New("password hash uses an unsupported argon2 version")
)

var (
	ErrStudentNotFound = errors.New("students: not found")
	ErrEmailTaken      = errors.New("students: email already in use")
)
