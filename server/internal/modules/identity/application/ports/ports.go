package ports

import (
	"context"
	"io"
	classescommand "quizzivy/internal/modules/classes/application/command"
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"time"
)

// GoogleProvider is the port to Google: the code exchange and the id-token
// verification, which fail for the domain's ErrGoogle* reasons.
type GoogleProvider interface {
	Exchange(ctx context.Context, code, codeVerifier, redirectURI string) (string, error)
	Verify(ctx context.Context, rawIDToken string) (model.GoogleIdentity, error)
}

// Principals is the access module's principal cache. A command that changes a
// user's access forgets the user on this machine, so the next request reads
// the user's current state and session epoch; a new session resolves the
// permissions the signed-in user is shown.
type Principals interface {
	Forget(userID string)
	Resolve(ctx context.Context, userID string) (access.Principal, error)
}

// ObjectStore keeps profile photos: the slice of the platform's object store
// this module uses. A nil store leaves photos unavailable.
type ObjectStore interface {
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Delete(ctx context.Context, key string) error
	SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// PhotoProcessor turns an upload into the square PNG that is stored. It reads
// at most its size limit from body and fails with the domain's ErrAvatar*
// reasons, having stored nothing.
type PhotoProcessor interface {
	Square(ctx context.Context, body io.Reader) ([]byte, error)
}

// SelfEnroller creates an account from a join code and enrols it (§6.3): the classes module's EnrolNewMember command.
type SelfEnroller = cqrs.CommandHandler[classescommand.EnrolNewMember, classesdomain.EnrolResult]
