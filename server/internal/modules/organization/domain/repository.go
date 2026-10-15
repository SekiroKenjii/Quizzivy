package domain

import (
	"context"
	"time"
)

// Save is a whole document to store over the revision it was read at, by
// whom and from where.
type Save struct {
	Settings         Settings
	ExpectedRevision int64
	ActorID          string
	IP               string
	UserAgent        string
	Now              time.Time
	// GoogleConfigured says whether this server can sign anyone in with Google at all; when it cannot, Google counts for nobody in the lockout judgement.
	GoogleConfigured bool
}

// Repository reads and writes the one settings row and judges a sign-in
// policy against the accounts.
type Repository interface {
	// Read returns the stored document.
	Read(ctx context.Context) (Settings, error)
	// Save stores in.Settings in one transaction that locks the row: it fails
	// with ErrStale when the stored revision is not in.ExpectedRevision, with
	// *LockedOut when the policy's Impact, judged inside the transaction for
	// in.ActorID, is a Lockout, and otherwise writes the document with the
	// revision one higher and an organization.updated audit row naming the
	// fields that changed. A document equal to the stored one writes nothing
	// and returns the stored document. It returns the document as stored.
	Save(ctx context.Context, in Save) (Settings, error)
	// Impact judges policy against the active accounts for actorID. Google
	// counts only when googleConfigured.
	Impact(ctx context.Context, policy SignIn, actorID string, googleConfigured bool) (Impact, error)
}
