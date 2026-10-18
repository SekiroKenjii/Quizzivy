package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
)

// EnableUser lets the disabled account ID sign in again.
type EnableUser struct {
	Request domain.WriteRequest
	ID      string
}

type EnableUserHandler struct {
	*support.Admin
}

// Handle returns the account, enabled. It applies the guards in order:
// domain.ErrUserNotFound and the subset rule (the access module's
// ErrTargetRoleNotAllowed). What the disable ended stays ended. An account
// enabled already is returned as it is.
func (h EnableUserHandler) Handle(ctx context.Context, cmd EnableUser) (domain.AdminUser, error) {
	err := actOnAccount(ctx, h.Admin, cmd.Request, cmd.ID, false, func(target access.Principal) error {
		return h.Repo.Enable(ctx, cmd.Request, accountWrite(h.Admin, cmd.ID, target))
	})
	if err != nil {
		return domain.AdminUser{}, err
	}
	return h.Repo.Get(ctx, cmd.ID)
}
