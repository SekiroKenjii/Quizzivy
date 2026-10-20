// Package worker creates the accounts of the user imports. It claims a queued
// job, creates the accounts of its rows 25 to a transaction under the
// importer's rights as they stand when each batch starts, and settles the job
// done, failed, or back in the queue.
package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	accessmodel "quizzivy/internal/modules/access/application/model"
	accessquery "quizzivy/internal/modules/access/application/query"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/domain"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	"quizzivy/internal/shared/access"
)

const (
	settleBudget = 10 * time.Second
	maxRoleReads = 3
)

var errNothingLeft = errors.New("user import: no row waits but the job is not finished")

type refusal struct{ code string }

func (r refusal) Error() string { return "user import: refused: " + r.code }

// UserImports executes the user imports one job at a time. It reads the
// importer's rights afresh at the start of every batch: the principal cache is
// per process, so a write the API made never reaches this one. A supervisor
// decides when it runs: RunOne claims at most one job and returns, and NextDue
// says when the next one falls due. A nil Logger logs nothing, a nil Now is the
// clock, a nil Passwords makes the app's temporary passwords, a nil Sealer
// means no sheet can be kept, a nil Links no invite can be sent, and a Timeout
// of zero is domain.ImportRunLimit.
type UserImports struct {
	Queue      domain.UserImportQueue
	Roles      ports.AssignableRole
	Principals ports.Principals
	Policy     ports.SignInPolicy
	Links      domain.SetPasswordLinks
	Sealer     ports.Sealer
	Passwords  func(ctx context.Context) (password, hash string, err error)
	Logger     *slog.Logger
	Now        func() time.Time
	Timeout    time.Duration

	service *support.Service
}

// NextDue reports when the queue will next have a job for this runner; false
// means none is waiting.
func (r UserImports) NextDue(ctx context.Context) (time.Time, bool, error) {
	return r.Queue.NextDue(ctx)
}

// RunOne claims the oldest waiting job and runs it to its end. It reports
// whether it found one. A job that fails is marked failed and is not an error
// of RunOne; an error is a failure to claim, to record the outcome, or a run
// that ended for a reason worth another try, which gives the job back to the
// queue with the attempt spent.
func (r UserImports) RunOne(ctx context.Context) (bool, error) {
	r = r.withDefaults()
	job, err := r.Queue.Claim(ctx)
	if errors.Is(err, domain.ErrNoImportWork) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	started := r.Now()
	work, cancel := context.WithTimeout(ctx, r.Timeout)
	done, runErr := r.execute(work, job)
	cancel()
	outcome, err := r.settle(ctx, job, runErr)
	fields := []any{"jobId", job.ID, "attempt", job.Attempts, "outcome", outcome,
		"created", done.Created, "skipped", done.Skipped, "durationMs", r.Now().Sub(started).Milliseconds()}
	if outcome == outcomeRetried {
		fields = append(fields, "failure", failureClass(runErr))
	}
	r.Logger.Info("user import", fields...)
	if err != nil {
		return true, err
	}
	if outcome == outcomeRetried {
		return false, runErr
	}
	return true, nil
}

const (
	outcomeDone     = "done"
	outcomeLost     = "lost"
	outcomeReleased = "released"
	outcomeRetried  = "retried"
)

func (r UserImports) withDefaults() UserImports {
	if r.Logger == nil {
		r.Logger = slog.New(slog.DiscardHandler)
	}
	if r.Now == nil {
		r.Now = time.Now
	}
	if r.Timeout <= 0 {
		r.Timeout = domain.ImportRunLimit
	}
	if r.Passwords == nil {
		r.Passwords = support.TemporaryPassword
	}
	r.service = &support.Service{Now: r.Now, Policy: r.Policy, Links: r.Links, Principals: r.Principals}
	return r
}

func (r UserImports) settle(ctx context.Context, job domain.ImportJob, runErr error) (string, error) {
	write, cancel := context.WithTimeout(context.WithoutCancel(ctx), settleBudget)
	defer cancel()
	var refused refusal
	switch {
	case runErr == nil:
		return outcomeDone, nil
	case errors.Is(runErr, domain.ErrImportLeaseLost):
		return outcomeLost, nil
	case errors.As(runErr, &refused):
		return refused.code, lost(r.Queue.Fail(write, job.ID, job.Attempts, refused.code))
	case ctx.Err() != nil:
		return outcomeReleased, lost(r.Queue.Release(write, job.ID, job.Attempts))
	default:
		return outcomeRetried, lost(r.Queue.Retry(write, job.ID, job.Attempts))
	}
}

func lost(err error) error {
	if errors.Is(err, domain.ErrImportLeaseLost) {
		return nil
	}
	return err
}

func (r UserImports) execute(ctx context.Context, job domain.ImportJob) (domain.ImportProgress, error) {
	var total domain.ImportProgress
	for {
		req, err := r.importer(ctx, job)
		if err != nil {
			return total, err
		}
		todo, err := r.Queue.Todo(ctx, job.ID, domain.ImportBatchSize)
		if err != nil {
			return total, err
		}
		progress, err := r.batch(ctx, job, req, todo)
		if err != nil {
			return total, err
		}
		total.Created += progress.Created
		total.Skipped += progress.Skipped
		if progress.Done {
			total.Done = true
			return total, nil
		}
		if len(todo) == 0 {
			return total, errNothingLeft
		}
	}
}

func (r UserImports) importer(ctx context.Context, job domain.ImportJob) (domain.WriteRequest, error) {
	r.Principals.Forget(job.CreatedBy)
	principal, err := r.Principals.Resolve(ctx, job.CreatedBy)
	switch {
	case errors.Is(err, accessdomain.ErrUnknownUser):
		return domain.WriteRequest{}, refusal{domain.ImportFailedNotAllowed}
	case err != nil:
		return domain.WriteRequest{}, err
	case principal.Disabled || !principal.Permissions.Has(access.PeopleUsersManage):
		return domain.WriteRequest{}, refusal{domain.ImportFailedNotAllowed}
	}
	return domain.WriteRequest{ActorID: job.CreatedBy, All: principal.Permissions.Has(access.ScopeAll), Grants: principal.Permissions}, nil
}

func (r UserImports) batch(ctx context.Context, job domain.ImportJob, req domain.WriteRequest, todo []domain.ImportTodo) (domain.ImportProgress, error) {
	policy, err := r.service.SignInPolicy(ctx)
	if err != nil {
		return domain.ImportProgress{}, err
	}
	if !r.deliverable(job.Delivery, policy) {
		return domain.ImportProgress{}, refusal{domain.ImportFailedDelivery}
	}
	kept, err := r.passwords(ctx, job, todo)
	if err != nil {
		return domain.ImportProgress{}, err
	}
	batch := domain.ImportBatch{
		JobID: job.ID, Attempt: job.Attempts, By: job.CreatedBy, Delivery: job.Delivery,
		Keep: !policy.ForcePasswordChange, Now: r.Now(),
	}
	for range maxRoleReads {
		if batch.Rows, err = r.creates(ctx, req, todo, kept); err != nil {
			return domain.ImportProgress{}, err
		}
		progress, err := r.Queue.Settle(ctx, batch, r.inviter(job))
		if errors.Is(err, domain.ErrRoleChanged) {
			continue
		}
		return progress, deliveryRefusal(err)
	}
	return domain.ImportProgress{}, domain.ErrRoleChanged
}

func (r UserImports) deliverable(delivery domain.ImportDelivery, policy organizationmodel.SignInPolicy) bool {
	switch delivery {
	case domain.ImportSheet:
		return r.Sealer != nil && policy.Password
	case domain.ImportEmailLink:
		return r.Links != nil
	}
	return false
}

func deliveryRefusal(err error) error {
	if errors.Is(err, domain.ErrSetPasswordUnavailable) || errors.Is(err, domain.ErrPasswordSignInDisabled) {
		return refusal{domain.ImportFailedDelivery}
	}
	return err
}

type kept struct {
	hash   string
	sealed []byte
}

func (r UserImports) passwords(ctx context.Context, job domain.ImportJob, todo []domain.ImportTodo) ([]kept, error) {
	out := make([]kept, len(todo))
	if job.Delivery != domain.ImportSheet {
		return out, nil
	}
	failures := make([]error, len(todo))
	var group sync.WaitGroup
	for i, row := range todo {
		group.Go(func() {
			password, hash, err := r.Passwords(ctx)
			if err != nil {
				failures[i] = err
				return
			}
			sealed, err := r.Sealer.Seal([]byte(password), domain.SheetAAD(job.ID, row.Ordinal))
			out[i], failures[i] = kept{hash: hash, sealed: sealed}, err
		})
	}
	group.Wait()
	return out, errors.Join(failures...)
}

func (r UserImports) creates(ctx context.Context, req domain.WriteRequest, todo []domain.ImportTodo, kept []kept) ([]domain.ImportCreate, error) {
	roles := map[string]accessmodel.AssignableRole{}
	out := make([]domain.ImportCreate, len(todo))
	for i, row := range todo {
		role, seen := roles[row.RoleID]
		if !seen {
			var err error
			role, err = r.Roles.Handle(ctx, accessquery.AssignableRole{By: support.Caller(req), RoleID: row.RoleID})
			if err != nil {
				return nil, roleRefusal(err)
			}
			roles[row.RoleID] = role
		}
		if row.ClassID != "" && (!role.StudentTarget || !req.All) {
			return nil, refusal{domain.ImportFailedNotAllowed}
		}
		out[i] = domain.ImportCreate{ImportTodo: row, RoleName: role.Name, RoleRevision: role.Revision, Hash: kept[i].hash, Secret: kept[i].sealed}
	}
	return out, nil
}

func roleRefusal(err error) error {
	if errors.Is(err, accessdomain.ErrRoleNotFound) || errors.Is(err, accessdomain.ErrNotAssignable) || errors.Is(err, accessdomain.ErrTargetRoleNotAllowed) {
		return refusal{domain.ImportFailedNotAllowed}
	}
	return err
}

func (r UserImports) inviter(job domain.ImportJob) domain.LinkIssue {
	if job.Delivery != domain.ImportEmailLink {
		return nil
	}
	invite := command.IssueSetPasswordLinkHandler{Service: r.service}
	return func(ctx context.Context, tx domain.LinkTx, userID string) error {
		_, err := invite.Handle(ctx, command.IssueSetPasswordLink{Tx: tx, UserID: userID, Purpose: domain.LinkInvite, By: job.CreatedBy})
		return err
	}
}
