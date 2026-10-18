package command

import (
	"context"
	"errors"
	"log/slog"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
	"sync"
)

func accountTarget(ctx context.Context, a *support.Admin, req domain.WriteRequest, id string, refuseSelf bool) (access.Principal, error) {
	target, err := a.Service.Principals.Resolve(ctx, id)
	if errors.Is(err, accessdomain.ErrUnknownUser) {
		return access.Principal{}, domain.ErrUserNotFound
	}
	if err != nil {
		return access.Principal{}, err
	}
	if !access.CanActOn(req.Grants, target.Permissions) {
		return access.Principal{}, accessdomain.ErrTargetRoleNotAllowed
	}
	if refuseSelf && id == req.ActorID {
		return access.Principal{}, domain.ErrCannotActOnSelf
	}
	return target, nil
}

func actOnAccount(ctx context.Context, a *support.Admin, req domain.WriteRequest, id string, refuseSelf bool, write func(target access.Principal) error) error {
	var err error
	for range maxRoleReads {
		var target access.Principal
		if target, err = accountTarget(ctx, a, req, id, refuseSelf); err != nil {
			return err
		}
		err = write(target)
		if !errors.Is(err, domain.ErrRoleChanged) {
			a.Students.ForgetAfterReset(id, err)
			return err
		}
		a.Service.Principals.Forget(id)
	}
	return err
}

func accountWrite(a *support.Admin, id string, target access.Principal) domain.AccountWrite {
	return domain.AccountWrite{UserID: id, RoleID: target.RoleID, Now: a.Service.Now()}
}

var refusedForAccount = []error{
	domain.ErrUserNotFound,
	accessdomain.ErrTargetRoleNotAllowed,
	domain.ErrCannotActOnSelf,
	domain.ErrLastAdmin,
	domain.ErrAccountDisabled,
	domain.ErrActionFailed,
}

func classifyFailure(logger *slog.Logger, id string, err error) error {
	for _, known := range refusedForAccount {
		if errors.Is(err, known) {
			return known
		}
	}
	logger.Error("account action failed", slog.String("user_id", id), slog.Any("err", err))
	return domain.ErrActionFailed
}

func guarded(logger *slog.Logger, id string, run func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.Error("account action panicked", slog.String("user_id", id), slog.Any("panic", recovered))
			err = domain.ErrActionFailed
		}
	}()
	return run()
}

func eachBounded(ctx context.Context, count, workers int, run func(i int)) {
	next := make(chan int)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for i := range next {
				run(i)
			}
		}()
	}
feed:
	for i := range count {
		select {
		case next <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(next)
	group.Wait()
}
