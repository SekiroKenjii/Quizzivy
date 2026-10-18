package command

import (
	"context"
	accessquery "quizzivy/internal/modules/access/application/query"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
)

// ChangeUserRole gives the account ID the role RoleID.
type ChangeUserRole struct {
	Request domain.WriteRequest
	ID      string
	RoleID  string
}

type ChangeUserRoleHandler struct {
	*support.Admin
}

// Handle returns the account with its new role. It applies the guards in
// order: domain.ErrUserNotFound, the subset rule against the account's role
// and then against the new one (the access module's ErrRoleNotFound,
// ErrNotAssignable for the Assistant and ErrTargetRoleNotAllowed),
// domain.ErrCannotActOnSelf, domain.ErrLastAdmin for the only active Admin
// and *domain.OwnsContentError for an account that owns classes or content
// moved to a role that opens no teacher workspace. It fails with
// domain.ErrRoleChanged when either role changed three times while the change
// was made. A refusal writes nothing; a change ends the account's sessions and
// forgets its cached principal.
func (h ChangeUserRoleHandler) Handle(ctx context.Context, cmd ChangeUserRole) (domain.AdminUser, error) {
	err := actOnAccount(ctx, h.Admin, cmd.Request, cmd.ID, true, func(target access.Principal) error {
		role, err := h.Roles.Handle(ctx, accessquery.AssignableRole{By: support.Caller(cmd.Request), RoleID: cmd.RoleID})
		if err != nil {
			return err
		}
		return h.Repo.ChangeRole(ctx, cmd.Request, domain.RoleChange{
			AccountWrite:   accountWrite(h.Admin, cmd.ID, target),
			ToRoleID:       role.ID,
			ToRoleName:     role.Name,
			ToRoleRevision: role.Revision,
			ToAdmin:        role.Builtin == access.BuiltinAdmin,
			ToOpensTeacher: role.OpensTeacher,
		})
	})
	if err != nil {
		return domain.AdminUser{}, err
	}
	return h.Repo.Get(ctx, cmd.ID)
}
