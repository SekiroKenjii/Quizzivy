package domain

import (
	"errors"
)

var ErrUserNotFound = errors.New("user not found")

var (
	ErrDisplayNameInvalid  = errors.New("display name is invalid")
	ErrPhoneInvalid        = errors.New("phone is invalid")
	ErrLocaleInvalid       = errors.New("locale is invalid")
	ErrTimeZoneInvalid     = errors.New("time zone is invalid")
	ErrProfileEmpty        = errors.New("profile patch is empty")
	ErrPreferencesInvalid  = errors.New("preferences shape is invalid")
	ErrPreferencesTooLarge = errors.New("preferences exceed the stored byte cap")
)

// ErrInvalidCredentials is returned for every failed login, whatever the actual
// cause: no such email, a Google-only account, a disabled account, or the wrong
// password.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrAccountChanged means the account was disabled or deleted, or its
// password or session epoch changed, between a sign-in's read of it and the
// storing of its refresh token.
var ErrAccountChanged = errors.New("account changed during sign-in")

// ErrPasswordUnchanged means the new password is the one it would replace: the
// verified current password, or, while the account must change its password,
// the temporary one.
var ErrPasswordUnchanged = errors.New("new password is the current one")

var (
	ErrAccountDisabled  = errors.New("account disabled")
	ErrNoPasswordSet    = errors.New("account has no password")
	ErrPasswordTooShort = errors.New("new password is too short")
	ErrPasswordTooLong  = errors.New("new password is too long")
	ErrNameRequired     = errors.New("full name is empty")
	ErrNameTooLong      = errors.New("full name is too long")
)

// ErrRefreshRejected covers every ordinary refresh failure: no cookie, an
// unknown token, an expired one, or a suspended account. A suspended account is
// not called out, for the same reason Login does not call it out.
var ErrRefreshRejected = errors.New("refresh rejected")

// ErrRefreshReused is §5.2 reuse detection firing: the presented token had
// already been rotated, and the whole family has now been revoked.
var ErrRefreshReused = errors.New("refresh token reused")

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

// ErrSessionNotFound means the caller has no live session with that id: it is
// unknown, another user's, revoked or expired, and the four read alike.
var ErrSessionNotFound = errors.New("session not found")

// ErrSessionIsCurrent means the session to end is the one making the request.
var ErrSessionIsCurrent = errors.New("session is the current one")

// ErrNoCurrentSession means the request carried no refresh cookie that names
// a live session of the caller, so the caller's own session cannot be told
// from the others and nothing is revoked.
var ErrNoCurrentSession = errors.New("no current session")

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
	ErrClassNotFound   = errors.New("students: class not found")
	ErrForbidden       = errors.New("students: the caller may not act on this account")
	ErrStudentShared   = errors.New("students: someone else also reaches this student")
	ErrLastAdmin       = errors.New("users: the last active Admin cannot be demoted, disabled or deleted")
)

var (
	ErrReferenced  = errors.New("student: retained history or assignments reference this resource")
	ErrNotArchived = errors.New("student: deactivate or close before deleting")
)
