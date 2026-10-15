package query

import (
	"context"
	"errors"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

// GetUser asks for one account for the Users sheet, as Request's caller may
// act on it.
type GetUser struct {
	Request domain.WriteRequest
	ID      string
}

// GetUserResult is the account with what the caller may do to it.
type GetUserResult struct {
	Detail domain.AdminUserDetail
	Can    domain.UserCan
}

type GetUserHandler struct {
	*support.Admin
}

// Handle fails with domain.ErrUserNotFound for an id that names no account.
func (h GetUserHandler) Handle(ctx context.Context, q GetUser) (GetUserResult, error) {
	detail, err := h.Repo.Detail(ctx, q.ID)
	if err != nil {
		return GetUserResult{}, err
	}
	target, err := h.Service.Principals.Resolve(ctx, q.ID)
	if errors.Is(err, accessdomain.ErrUnknownUser) {
		return GetUserResult{}, domain.ErrUserNotFound
	}
	if err != nil {
		return GetUserResult{}, err
	}
	if detail.User.AvatarURL, err = h.Service.AvatarLink(ctx, detail.User.AvatarKey); err != nil {
		return GetUserResult{}, err
	}
	return GetUserResult{Detail: detail, Can: detail.Can(q.Request, target.Permissions)}, nil
}
