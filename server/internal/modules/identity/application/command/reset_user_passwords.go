package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/validation"

	"github.com/google/uuid"
)

// hashWorkers is how many temporary passwords a bulk reset hashes at once, and
// so how many of the process's password-hash slots one call can hold.
const hashWorkers = 2

// ResetUserPassword resets the password of the account ID as Delivery says.
type ResetUserPassword struct {
	Request  domain.WriteRequest
	ID       string
	Delivery domain.Delivery
}

// ResetUserPasswordResult holds the temporary password for
// domain.DeliveryTemporaryPassword, which is shown once and kept nowhere.
type ResetUserPasswordResult struct {
	TemporaryPassword string
}

type ResetUserPasswordHandler struct {
	*support.Admin
}

// Handle fails with domain.ErrPasswordSignInDisabled while the centre has
// turned password sign-in off, with domain.ErrSetPasswordUnavailable for a
// link on a server that cannot send one, and then applies the guards in order:
// domain.ErrUserNotFound, the subset rule (the access module's
// ErrTargetRoleNotAllowed), domain.ErrCannotActOnSelf and
// domain.ErrAccountDisabled for a disabled account. A temporary password is
// set, or the password is cleared and a link to choose another is issued on
// the transaction that clears it; either way the account's sessions end, its
// unused set-password links are revoked and its cached principal is forgotten.
// It fails with domain.ErrRoleChanged when the account's role changed three
// times while the reset was made. A refusal writes nothing.
func (h ResetUserPasswordHandler) Handle(ctx context.Context, cmd ResetUserPassword) (ResetUserPasswordResult, error) {
	req, err := beginReset(ctx, h.Admin, cmd.Request, cmd.Delivery)
	if err != nil {
		return ResetUserPasswordResult{}, err
	}
	reset, err := resetAccount(ctx, h.Admin, req, cmd.ID, cmd.Delivery)
	if err != nil {
		return ResetUserPasswordResult{}, err
	}
	return ResetUserPasswordResult{TemporaryPassword: reset.TemporaryPassword}, nil
}

// ResetUsersPasswords resets the passwords of the accounts IDs name, up to
// domain.MaxBulkPasswordReset for domain.DeliveryTemporaryPassword and
// domain.MaxBulkLinkReset for domain.DeliveryEmailLink.
type ResetUsersPasswords struct {
	Request  domain.WriteRequest
	IDs      []string
	Delivery domain.Delivery
}

type ResetUsersPasswordsHandler struct {
	*support.Admin
}

// Handle resets each account as ResetUserPassword does, in a transaction of its
// own, so a batch cannot do what one reset cannot. It fails as that does for
// the whole request, and with a *validation.Error naming userIds for more
// accounts than the delivery allows. It never fails once an account is reset,
// because its answer is the only place the temporary passwords exist: an
// account it cannot reset is reported in the result with the refusal
// ResetUserPassword would have answered, or domain.ErrActionFailed for a fault
// with that account alone, which is logged. Temporary passwords are hashed
// two at a time. When ctx ends no further account is started and the result
// holds what was done. The audit rows of one call carry one batch id, made
// when the call starts.
func (h ResetUsersPasswordsHandler) Handle(ctx context.Context, cmd ResetUsersPasswords) (domain.BulkUserReset, error) {
	limit, workers := domain.MaxBulkLinkReset, 1
	if cmd.Delivery == domain.DeliveryTemporaryPassword {
		limit, workers = domain.MaxBulkPasswordReset, hashWorkers
	}
	if len(cmd.IDs) > limit {
		var refused validation.Error
		refused.Add("userIds", fmt.Sprintf("Chỉ đặt lại được tối đa %d tài khoản cùng lúc theo cách này.", limit))
		return domain.BulkUserReset{}, &refused
	}
	req, err := beginReset(ctx, h.Admin, cmd.Request, cmd.Delivery)
	if err != nil {
		return domain.BulkUserReset{}, err
	}
	batch, err := uuid.NewV7()
	if err != nil {
		return domain.BulkUserReset{}, fmt.Errorf("make a batch id: %w", err)
	}
	req.BatchID = batch.String()

	type outcome struct {
		attempted bool
		reset     domain.UserReset
		err       error
	}
	outcomes := make([]outcome, len(cmd.IDs))
	eachBounded(ctx, len(cmd.IDs), workers, func(i int) {
		id := cmd.IDs[i]
		outcomes[i].attempted = true
		outcomes[i].err = guarded(h.Students.Logger, id, func() error {
			var err error
			outcomes[i].reset, err = resetAccount(ctx, h.Admin, req, id, cmd.Delivery)
			return err
		})
	})

	var result domain.BulkUserReset
	attempted := 0
	for i, o := range outcomes {
		switch {
		case !o.attempted:
		case o.err != nil:
			attempted++
			result.Failed = append(result.Failed, domain.UserFailure{UserID: cmd.IDs[i], Reason: classifyFailure(h.Students.Logger, cmd.IDs[i], o.err)})
		default:
			attempted++
			result.Reset = append(result.Reset, o.reset)
		}
	}
	if ctx.Err() != nil {
		h.Students.Logger.Warn("bulk password reset cut short",
			slog.Int("named", len(cmd.IDs)), slog.Int("attempted", attempted), slog.Int("reset", len(result.Reset)))
	}
	return result, nil
}

func beginReset(ctx context.Context, a *support.Admin, req domain.WriteRequest, delivery domain.Delivery) (domain.WriteRequest, error) {
	if !delivery.Valid() {
		return domain.WriteRequest{}, errors.New("reset user password: unknown delivery")
	}
	policy, err := a.Service.SignInPolicy(ctx)
	if err != nil {
		return domain.WriteRequest{}, err
	}
	if !policy.Password {
		return domain.WriteRequest{}, domain.ErrPasswordSignInDisabled
	}
	if delivery == domain.DeliveryEmailLink && a.Service.Links == nil {
		return domain.WriteRequest{}, domain.ErrSetPasswordUnavailable
	}
	req.KeepTemporaryPassword = !policy.ForcePasswordChange
	return req, nil
}

func resetAccount(ctx context.Context, a *support.Admin, req domain.WriteRequest, id string, delivery domain.Delivery) (domain.UserReset, error) {
	var done domain.UserReset
	err := actOnAccount(ctx, a, req, id, true, func(target access.Principal) error {
		user, err := a.Repo.Get(ctx, id)
		if err != nil {
			return err
		}
		if user.Status == domain.AccountDisabled {
			return domain.ErrAccountDisabled
		}
		reset := domain.AccountPasswordReset{AccountWrite: accountWrite(a, id, target), Delivery: delivery}
		switch delivery {
		case domain.DeliveryTemporaryPassword:
			if done.TemporaryPassword, reset.Hash, err = a.Students.Password(ctx); err != nil {
				return err
			}
		case domain.DeliveryEmailLink:
			reset.Issue = func(ctx context.Context, tx domain.LinkTx, userID string) error {
				_, err := IssueSetPasswordLinkHandler{Service: a.Service}.Handle(ctx, IssueSetPasswordLink{
					Tx: tx, UserID: userID, Purpose: domain.LinkReset, By: req.ActorID, KeepSessions: true,
				})
				return err
			}
		}
		if err := a.Repo.ResetPassword(ctx, req, reset); err != nil {
			return err
		}
		done.UserID, done.FullName, done.Email = id, user.FullName, user.Email
		return nil
	})
	if err != nil {
		return domain.UserReset{}, err
	}
	return done, nil
}
