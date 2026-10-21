package ports

import (
	"context"
	"io"
	accessmodel "quizzivy/internal/modules/access/application/model"
	accessquery "quizzivy/internal/modules/access/application/query"
	classescommand "quizzivy/internal/modules/classes/application/command"
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/domain"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	organizationquery "quizzivy/internal/modules/organization/application/query"
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

// AvatarObjects is the slice of the object store the avatar sweep uses: the
// keys under a prefix, at most limit of them in key order, and a delete.
type AvatarObjects interface {
	List(ctx context.Context, prefix string, limit int) ([]string, error)
	Delete(ctx context.Context, key string) error
}

// PhotoProcessor turns an upload into the square PNG that is stored. It reads
// at most its size limit from body and fails with the domain's ErrAvatar*
// reasons, having stored nothing.
type PhotoProcessor interface {
	Square(ctx context.Context, body io.Reader) ([]byte, error)
}

// SelfEnroller creates an account from a join code and enrols it (§6.3): the classes module's EnrolNewMember command.
type SelfEnroller = cqrs.CommandHandler[classescommand.EnrolNewMember, classesdomain.EnrolResult]

// SignInPolicy is how the centre lets people sign in: the organization module's SignInPolicy query, answered from a snapshot at most 30 seconds old.
type SignInPolicy = cqrs.QueryHandler[organizationquery.SignInPolicy, organizationmodel.SignInPolicy]

// AssignableRole is the access module's AssignableRole query: whether the caller may give a role to a user, and what the role is.
type AssignableRole = cqrs.QueryHandler[accessquery.AssignableRole, accessmodel.AssignableRole]

// RoleChoices is the access module's RoleChoices query: every role with whether the caller may give it.
type RoleChoices = cqrs.QueryHandler[accessquery.RoleChoices, []accessmodel.RoleChoice]

// TableReader reads the first sheet of a workbook, or a CSV file, into memory.
// It fails with the platform's tabular errors, which the transport words.
type TableReader interface {
	Read(file []byte, format domain.ImportFormat) (domain.ImportSource, error)
}

// EmailBudget answers how many more messages the centre can send today.
type EmailBudget interface {
	Remaining(ctx context.Context, now time.Time) (model.EmailAllowance, error)
}

// Wake tells the import worker that a user import is waiting. It never blocks.
type Wake interface {
	Wake()
}

// Sealer seals the temporary passwords a hand-out sheet holds until it is
// downloaded, and opens them again; each is bound to data naming its row.
type Sealer interface {
	Seal(plaintext, aad []byte) ([]byte, error)
	Open(sealed, aad []byte) ([]byte, error)
}

// ContentMover hands a disabled account's content to another account in one
// transaction: it locks both accounts, moves the classes, tests, questions,
// question groups, media and assignments, and writes the audit row. It fails
// with the domain's TransferLedger errors, and with ErrWriteContended when the
// database ended the transaction to break a deadlock or a serialization
// conflict; nothing moved then.
type ContentMover interface {
	Move(ctx context.Context, req domain.WriteRequest, in domain.ContentTransfer) (domain.TransferredContent, error)
}

// Notifier is the notifications module's Notify, which tells the recipient of
// a transfer what they were given.
type Notifier = cqrs.CommandHandler[notificationscommand.Notify, cqrs.Nothing]
