package command

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
	"time"
)

// Extend moves the close of an assignment that has not closed later by
// Minutes, for everyone. When Notify is set, the students whose close it
// moved are told of their new one once it has committed.
type Extend struct {
	Request domain.Request
	Minutes int
	Notify  bool
	Now     time.Time
}

type ExtendHandler struct {
	*support.Service
}

func (s ExtendHandler) Handle(ctx context.Context, cmd Extend) (domain.Assignment, error) {
	extended, err := s.Repo.Extend(ctx, cmd.Request, cmd.Minutes, cmd.Notify, cmd.Now)
	if err != nil {
		return extended, err
	}
	if cmd.Notify {
		s.TellMoved(ctx, extended.ID)
	}
	return extended, nil
}
