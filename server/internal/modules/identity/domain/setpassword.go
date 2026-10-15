package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"quizzivy/internal/shared/audit"
	"time"
)

// SetPasswordPurpose says why a set-password link was issued.
type SetPasswordPurpose string

const (
	// LinkInvite carries a new account's first password.
	LinkInvite SetPasswordPurpose = "invite"
	// LinkReset carries a new password for an account that has one or had one.
	LinkReset SetPasswordPurpose = "reset"
)

// Valid reports whether p is one of the two purposes a link is issued for.
func (p SetPasswordPurpose) Valid() bool { return p == LinkInvite || p == LinkReset }

// SetPasswordLifetime is how long a link works from the moment it is issued.
const SetPasswordLifetime = 24 * time.Hour

const setPasswordTokenBytes = 32

// NewSetPasswordToken returns a link's secret, 32 random bytes in unpadded
// base64url, and its SHA-256, which is all the database keeps of it.
func NewSetPasswordToken() (token string, hash []byte, err error) {
	raw := make([]byte, setPasswordTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate set-password token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashSetPasswordToken(token), nil
}

// HashSetPasswordToken is the SHA-256 of token, the form a link is stored and
// looked up by.
func HashSetPasswordToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// SetPasswordLink is a stored link as a lookup by its token finds it, in any
// state, with the account it is for.
type SetPasswordLink struct {
	UserID          string
	Email           string
	FullName        string
	Purpose         SetPasswordPurpose
	ExpiresAt       time.Time
	UsedAt          *time.Time
	RevokedAt       *time.Time
	AccountDisabled bool
}

// Usable reports whether the link can still set a password at now: it is
// unused, unrevoked and unexpired, and its account is not disabled. A link
// that is not usable is invalid for the same reader whichever of those it
// failed, so nothing says which.
func (l SetPasswordLink) Usable(now time.Time) bool {
	return l.UsedAt == nil && l.RevokedAt == nil && now.Before(l.ExpiresAt) && !l.AccountDisabled
}

// NewLink is a link to store: the hash of its token, for an account, issued at
// Now and expiring SetPasswordLifetime later. CreatedBy is the user who asked
// for it, empty when a job did.
type NewLink struct {
	UserID    string
	TokenHash []byte
	Purpose   SetPasswordPurpose
	CreatedBy string
	Now       time.Time
}

// LinkRecipient is the account a link is for, as the transaction issuing it
// reads it.
type LinkRecipient struct {
	ID       string
	Email    string
	FullName string
	Disabled bool
}

// LinkMailKind is which message carries the outcome of issuing a link.
type LinkMailKind int

const (
	// MailAccountInvite is an invite's link: the account's first password.
	MailAccountInvite LinkMailKind = iota + 1
	// MailPasswordReset is a reset's link.
	MailPasswordReset
	// MailSignInInvite tells a new account to sign in with Google, for a centre that has turned password sign-in off. It carries no token.
	MailSignInInvite
)

// LinkMail is the message a link issue enqueues. Token is the link's secret
// and is empty for MailSignInInvite.
type LinkMail struct {
	Kind     LinkMailKind
	UserID   string
	To       string
	FullName string
	Token    string
}

// IssuedLink is what issuing a link did. SignInInvite is true when the centre
// has turned password sign-in off and the account was sent a sign-in invite
// and no link; ExpiresAt is then zero.
type IssuedLink struct {
	ExpiresAt    time.Time
	SignInInvite bool
}

// ClaimLink asks to use a link: set the password of the account its token
// names to NewHash, as of Now.
type ClaimLink struct {
	TokenHash []byte
	NewHash   string
	Now       time.Time
	IP        *string
	UserAgent *string
}

// ClaimedLink is the account whose password a claim set, and why its link was
// issued.
type ClaimedLink struct {
	UserID  string
	Purpose SetPasswordPurpose
}

// LinkTx is the writes of issuing a link, run in a transaction that belongs to
// the caller, so that the account and its link commit together or not at all.
type LinkTx interface {
	// Recipient reads the account as this transaction sees it, an account
	// created in it included, or returns ErrUserNotFound.
	Recipient(ctx context.Context, userID string) (LinkRecipient, error)
	// RevokeUnused ends the account's unused links, so the one issued next is
	// the only one that works.
	RevokeUnused(ctx context.Context, userID string, at time.Time) error
	// Insert stores a link.
	Insert(ctx context.Context, link NewLink) error
	// EndSessions revokes every refresh family of the account and moves its
	// session epoch, so no access token it holds outlives the transaction.
	EndSessions(ctx context.Context, userID string, at time.Time) error
	// Enqueue writes the message that carries the link to the email outbox, or
	// returns ErrSetPasswordUnavailable when this server cannot seal its secret.
	Enqueue(ctx context.Context, mail LinkMail) error
	// Audit appends one audit row.
	Audit(ctx context.Context, entry audit.Entry) error
	// AfterCommit runs fn once the transaction has committed, and never when it
	// rolls back. It is where a cache is told and the dispatcher woken.
	AfterCommit(fn func())
}

// SetPasswordLinks persists links.
type SetPasswordLinks interface {
	// InTx runs fn in one transaction, committed when it returns nil, and then
	// the functions fn registered with AfterCommit.
	InTx(ctx context.Context, fn func(ctx context.Context, tx LinkTx) error) error
	// Find reads the link a token names in whatever state it is in, or returns
	// ErrSetPasswordLinkInvalid when none is stored.
	Find(ctx context.Context, tokenHash []byte) (SetPasswordLink, error)
	// Claim uses the link, sets the password and ends the account's other
	// links and sessions in one transaction, or returns
	// ErrSetPasswordLinkInvalid, having changed nothing, when the link cannot
	// be used. Of two claims of one link, one wins.
	Claim(ctx context.Context, in ClaimLink) (ClaimedLink, error)
}
