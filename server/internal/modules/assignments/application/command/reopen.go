package command

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
	"time"
)

type Reopen struct {
	Request  domain.Request
	ClosesAt time.Time
	Reason   string
	Now      time.Time
}

type ReopenHandler struct {
	*support.Service
}

func (s ReopenHandler) Handle(ctx context.Context, cmd Reopen) (domain.Assignment, error) {
	return s.Repo.Reopen(ctx, cmd.Request, cmd.ClosesAt, cmd.Reason, cmd.Now)
}
