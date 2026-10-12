package command

import (
	"context"
	"errors"
	"time"

	"quizzivy/internal/modules/notifications/application/internal/support"
	"quizzivy/internal/modules/notifications/domain"
)

// MaterialiseDue writes the notifications UserID has earned by the passing of
// time up to Now: an assignment that opened, a close a day or an hour away, a
// close an hour away that students have not met, a result that a close
// released. There is no timer behind it; it runs when the user reads their
// notifications, and at most once in every five minutes for one user in this
// process. A moment older than seven days is not made up, a notification the
// user already holds is left as it is, and one whose switch is off is not
// written. It answers how many it wrote.
type MaterialiseDue struct {
	UserID string
	Now    time.Time
}

type MaterialiseDueHandler struct {
	*support.Service
}

func (s MaterialiseDueHandler) Handle(ctx context.Context, cmd MaterialiseDue) (int, error) {
	if cmd.UserID == "" {
		return 0, domain.ErrNoRecipient
	}
	if !s.Throttle.Allow(cmd.UserID, cmd.Now) {
		return 0, nil
	}
	due, err := s.Repo.Due(ctx, cmd.UserID, cmd.Now)
	if err != nil {
		s.Throttle.Forget(cmd.UserID)
		return 0, err
	}
	writable, refused := splitValid(due)
	if len(writable) == 0 {
		return 0, errors.Join(refused...)
	}
	written, err := s.Repo.InsertAbsent(ctx, cmd.UserID, writable)
	if err != nil {
		s.Throttle.Forget(cmd.UserID)
		return 0, err
	}
	return written, errors.Join(refused...)
}

func splitValid(notices []domain.Notice) ([]domain.Notice, []error) {
	writable := make([]domain.Notice, 0, len(notices))
	var refused []error
	for _, notice := range notices {
		if err := notice.Validate(); err != nil {
			refused = append(refused, err)
			continue
		}
		writable = append(writable, notice)
	}
	return writable, refused
}
