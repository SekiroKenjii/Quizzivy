package command

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"

	"github.com/google/uuid"
)

// DisableUser disables the account ID.
type DisableUser struct {
	Request domain.WriteRequest
	ID      string
}

type DisableUserHandler struct {
	*support.Admin
}

// Handle returns the account, disabled. It applies the guards in order:
// domain.ErrUserNotFound, the subset rule (the access module's
// ErrTargetRoleNotAllowed), domain.ErrCannotActOnSelf and domain.ErrLastAdmin.
// The account's sessions end, its unused set-password links are revoked and the
// messages waiting for it are cancelled in the same transaction, after which
// its cached principal is forgotten. An account disabled already is returned as
// it is.
func (h DisableUserHandler) Handle(ctx context.Context, cmd DisableUser) (domain.AdminUser, error) {
	if err := disable(ctx, h.Admin, cmd.Request, cmd.ID); err != nil {
		return domain.AdminUser{}, err
	}
	return h.Repo.Get(ctx, cmd.ID)
}

// DisableUsers disables the accounts IDs name, up to domain.MaxBulkDisable.
type DisableUsers struct {
	Request domain.WriteRequest
	IDs     []string
}

type DisableUsersHandler struct {
	*support.Admin
}

// Handle disables each account as DisableUser does, one after the other and in
// a transaction of its own, so a batch cannot do what one disable cannot. An
// account it cannot disable is reported in the result with the refusal
// DisableUser would have answered, or domain.ErrActionFailed for a fault with
// that account alone, which is logged; the others are still disabled. When ctx
// ends no further account is started. The audit rows of one call carry one
// batch id, made when the call starts.
func (h DisableUsersHandler) Handle(ctx context.Context, cmd DisableUsers) (domain.BulkDisable, error) {
	batch, err := uuid.NewV7()
	if err != nil {
		return domain.BulkDisable{}, fmt.Errorf("make a batch id: %w", err)
	}
	req := cmd.Request
	req.BatchID = batch.String()
	var result domain.BulkDisable
	for _, id := range cmd.IDs {
		if ctx.Err() != nil {
			break
		}
		err := guarded(h.Students.Logger, id, func() error { return disable(ctx, h.Admin, req, id) })
		if err != nil {
			result.Failed = append(result.Failed, domain.UserFailure{UserID: id, Reason: classifyFailure(h.Students.Logger, id, err)})
			continue
		}
		result.Disabled = append(result.Disabled, id)
	}
	return result, nil
}

func disable(ctx context.Context, a *support.Admin, req domain.WriteRequest, id string) error {
	return actOnAccount(ctx, a, req, id, true, func(target access.Principal) error {
		return a.Repo.Disable(ctx, req, accountWrite(a, id, target))
	})
}
