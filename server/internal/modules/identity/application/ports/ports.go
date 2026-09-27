package ports

import (
	"context"
	classescommand "quizzivy/internal/modules/classes/application/command"
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/shared/cqrs"
)

// GoogleProvider is the port to Google: the code exchange and the id-token
// verification, which fail for the domain's ErrGoogle* reasons.
type GoogleProvider interface {
	Exchange(ctx context.Context, code, codeVerifier, redirectURI string) (string, error)
	Verify(ctx context.Context, rawIDToken string) (model.GoogleIdentity, error)
}

// Principals is the access module's principal cache. A command that changes a
// user's access forgets the user on this machine, so the next request reads
// the user's current state and session epoch.
type Principals interface {
	Forget(userID string)
}

// SelfEnroller creates an account from a join code and enrols it (§6.3): the classes module's EnrolNewMember command.
type SelfEnroller = cqrs.CommandHandler[classescommand.EnrolNewMember, classesdomain.EnrolResult]
