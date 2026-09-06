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

// SelfEnroller creates an account from a join code and enrols it (§6.3): the classes module's EnrolNewMember command.
type SelfEnroller = cqrs.CommandHandler[classescommand.EnrolNewMember, classesdomain.EnrolResult]
