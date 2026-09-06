package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/opt"
)

type EnrolExisting struct {
	UserID string
	Code   string
	Meta   domain.Meta
}

type EnrolExistingHandler struct {
	*support.Enrolment
}

func (s EnrolExistingHandler) Handle(ctx context.Context, cmd EnrolExisting) (domain.EnrolResult, error) {
	return s.Repo.Enrol(ctx, domain.EnrolInput{
		RawCode:        cmd.Code,
		ExistingUserID: cmd.UserID,
		Now:            s.Now(),
		IP:             opt.String(cmd.Meta.IP),
		UserAgent:      opt.String(cmd.Meta.UserAgent),
	})
}
