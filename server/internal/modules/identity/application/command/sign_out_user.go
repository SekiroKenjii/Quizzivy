package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

// SignOutUser ends every session the account ID has.
type SignOutUser struct {
	Request domain.WriteRequest
	ID      string
}

type SignOutUserHandler struct {
	*support.Admin
}

// Handle revokes every refresh family of the account and moves its session
// epoch, then forgets its cached principal. It applies the guards in order:
// domain.ErrUserNotFound and the subset rule (the access module's
// ErrTargetRoleNotAllowed). The caller may sign themselves out.
func (h SignOutUserHandler) Handle(ctx context.Context, cmd SignOutUser) (cqrs.Nothing, error) {
	err := actOnAccount(ctx, h.Admin, cmd.Request, cmd.ID, false, func(target access.Principal) error {
		return h.Repo.SignOut(ctx, cmd.Request, accountWrite(h.Admin, cmd.ID, target))
	})
	return cqrs.Nothing{}, err
}
