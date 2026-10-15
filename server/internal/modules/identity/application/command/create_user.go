package command

import (
	"context"
	"errors"
	accessquery "quizzivy/internal/modules/access/application/query"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/validation"
)

// CreateUser adds an account of any role the caller may give. Input carries
// the email, the full name, the role, the class and the delivery; the rest of
// it is the command's to fill.
type CreateUser struct {
	Request domain.WriteRequest
	Input   domain.NewUser
}

// CreateUserResult is the account created, and the temporary password for
// domain.DeliveryTemporaryPassword, which is shown once and kept nowhere.
type CreateUserResult struct {
	User              domain.AdminUser
	TemporaryPassword string
}

type CreateUserHandler struct {
	*support.Admin
}

// Handle fails with a *validation.Error naming fullName for a name that
// domain.CleanFullName refuses and email for an address that holds a character
// domain.HiddenInEmail finds, with the access module's ErrRoleNotFound,
// ErrNotAssignable and ErrTargetRoleNotAllowed for a role the caller may not
// give, whether the role was that way when the call began or became so while
// it ran, with domain.ErrClassNotAllowed for a class given to a role that is
// not a student target or by a caller without scope.all, with
// domain.ErrClassNotFound, domain.ErrEmailTaken,
// domain.ErrPasswordSignInDisabled for a temporary password while password
// sign-in is off, and domain.ErrSetPasswordUnavailable for an email link on a
// server that cannot send one, and with domain.ErrRoleChanged when the role
// changed three times while the account was being created. A refusal writes
// nothing.
func (h CreateUserHandler) Handle(ctx context.Context, cmd CreateUser) (CreateUserResult, error) {
	in, req := cmd.Input, cmd.Request
	var refused validation.Error
	in.FullName = cleanName(&refused, in.FullName)
	checkAddress(&refused, in.Email)
	if invalid := refused.OrNil(); invalid != nil {
		return CreateUserResult{}, invalid
	}
	if !in.Delivery.Valid() {
		return CreateUserResult{}, errors.New("create user: unknown delivery")
	}
	if err := h.give(ctx, req, &in); err != nil {
		return CreateUserResult{}, err
	}
	in.Now = h.Service.Now()

	policy, err := h.Service.SignInPolicy(ctx)
	if err != nil {
		return CreateUserResult{}, err
	}
	var temporary string
	var issue domain.LinkIssue
	switch in.Delivery {
	case domain.DeliveryTemporaryPassword:
		if !policy.Password {
			return CreateUserResult{}, domain.ErrPasswordSignInDisabled
		}
		temporary, in.Hash, err = h.Students.Password(ctx)
		if err != nil {
			return CreateUserResult{}, err
		}
		req.KeepTemporaryPassword = !policy.ForcePasswordChange
	case domain.DeliveryEmailLink:
		if h.Service.Links == nil {
			return CreateUserResult{}, domain.ErrSetPasswordUnavailable
		}
		invite := IssueSetPasswordLinkHandler{Service: h.Service}
		issue = func(ctx context.Context, tx domain.LinkTx, userID string) error {
			_, err := invite.Handle(ctx, IssueSetPasswordLink{Tx: tx, UserID: userID, Purpose: domain.LinkInvite, By: req.ActorID})
			return err
		}
	}
	user, err := h.create(ctx, req, in, issue)
	if err != nil {
		return CreateUserResult{}, err
	}
	return CreateUserResult{User: user, TemporaryPassword: temporary}, nil
}

const maxRoleReads = 3

func (h CreateUserHandler) create(ctx context.Context, req domain.WriteRequest, in domain.NewUser, issue domain.LinkIssue) (domain.AdminUser, error) {
	user, err := h.Repo.Create(ctx, req, in, issue)
	for range maxRoleReads - 1 {
		if !errors.Is(err, domain.ErrRoleChanged) {
			break
		}
		if err = h.give(ctx, req, &in); err != nil {
			return domain.AdminUser{}, err
		}
		user, err = h.Repo.Create(ctx, req, in, issue)
	}
	return user, err
}

func (h CreateUserHandler) give(ctx context.Context, req domain.WriteRequest, in *domain.NewUser) error {
	role, err := h.Roles.Handle(ctx, accessquery.AssignableRole{By: support.Caller(req), RoleID: in.RoleID})
	if err != nil {
		return err
	}
	if in.ClassID != "" && (!role.StudentTarget || !req.All) {
		return domain.ErrClassNotAllowed
	}
	in.RoleName, in.RoleRevision = role.Name, role.Revision
	return nil
}
