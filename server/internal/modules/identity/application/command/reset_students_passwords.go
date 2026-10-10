package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"sync"
)

// resetWorkers is how many students a bulk reset works on at once, and so how
// many of the process's password-hash slots one call can hold.
const resetWorkers = 2

// ResetStudentsPasswords resets the passwords of the students IDs name, up to
// domain.MaxBulkReset, as ResetStudentPassword resets one: each student passes
// the same guards in the same order and is reset in a transaction of its own,
// which revokes the student's refresh families and moves the session epoch,
// after which the student's cached principal is forgotten.
type ResetStudentsPasswords struct {
	Request domain.WriteRequest
	IDs     []string
}

type ResetStudentsPasswordsHandler struct {
	*support.Students
}

type resetOutcome struct {
	attempted bool
	reset     domain.PasswordReset
	err       error
}

// Handle never fails once a student is reset, because its answer is the only
// place the temporary passwords exist. A student that cannot be reset is
// reported in the result; a student whose reset hit a fault is reported as
// domain.ErrResetFailed and logged with its id and the cause. When ctx ends,
// no further student is started and the result holds what was done.
func (s ResetStudentsPasswordsHandler) Handle(ctx context.Context, cmd ResetStudentsPasswords) (domain.BulkReset, error) {
	outcomes := make([]resetOutcome, len(cmd.IDs))
	next := make(chan int)
	var workers sync.WaitGroup
	for range resetWorkers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range next {
				outcomes[i] = s.resetOne(ctx, cmd.Request, cmd.IDs[i])
			}
		}()
	}
feed:
	for i := range cmd.IDs {
		select {
		case next <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(next)
	workers.Wait()

	var result domain.BulkReset
	attempted := 0
	for i, outcome := range outcomes {
		switch {
		case !outcome.attempted:
		case outcome.err != nil:
			attempted++
			result.Failed = append(result.Failed, domain.ResetFailure{StudentID: cmd.IDs[i], Reason: outcome.err})
		default:
			attempted++
			result.Reset = append(result.Reset, outcome.reset)
		}
	}
	if ctx.Err() != nil {
		s.Logger.Warn("bulk password reset cut short",
			slog.Int("named", len(cmd.IDs)), slog.Int("attempted", attempted), slog.Int("reset", len(result.Reset)))
	}
	return result, nil
}

func (s ResetStudentsPasswordsHandler) resetOne(ctx context.Context, req domain.WriteRequest, id string) (outcome resetOutcome) {
	outcome.attempted = true
	defer func() {
		if recovered := recover(); recovered != nil {
			s.Logger.Error("password reset panicked", slog.String("student_id", id), slog.Any("panic", recovered))
			outcome.err = domain.ErrResetFailed
		}
	}()

	student, err := s.Reach(ctx, req, id, false)
	if err == nil && student.DisabledAt != nil {
		err = domain.ErrStudentNotFound
	}
	if err != nil {
		outcome.err = s.classify(id, err)
		return outcome
	}
	password, hash, err := s.Password(ctx)
	if err != nil {
		outcome.err = s.classify(id, fmt.Errorf("make a temporary password: %w", err))
		return outcome
	}
	if err := s.Repo.ResetPassword(ctx, req, id, hash, s.Now()); err != nil {
		outcome.err = s.classify(id, err)
		return outcome
	}
	s.Principals.Forget(id)
	outcome.reset = domain.PasswordReset{StudentID: id, FullName: student.FullName, Email: student.Email, TemporaryPassword: password}
	return outcome
}

func (s ResetStudentsPasswordsHandler) classify(id string, err error) error {
	for _, known := range []error{domain.ErrStudentNotFound, domain.ErrForbidden, domain.ErrStudentShared} {
		if errors.Is(err, known) {
			return known
		}
	}
	s.Logger.Error("password reset failed", slog.String("student_id", id), slog.Any("err", err))
	return domain.ErrResetFailed
}
