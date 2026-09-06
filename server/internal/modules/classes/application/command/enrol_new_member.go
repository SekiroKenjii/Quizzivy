package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/opt"
)

// EnrolNewMember creates an account and enrols it (§6.3). The signup path.
type EnrolNewMember struct {
	Member domain.NewMember
	Code   string
	Meta   domain.Meta
}

type EnrolNewMemberHandler struct {
	*support.Enrolment
}

func (s EnrolNewMemberHandler) Handle(ctx context.Context, cmd EnrolNewMember) (domain.EnrolResult, error) {
	return s.Repo.Enrol(ctx, domain.EnrolInput{
		RawCode:   cmd.Code,
		NewMember: &cmd.Member,
		Now:       s.Now(),
		IP:        opt.String(cmd.Meta.IP),
		UserAgent: opt.String(cmd.Meta.UserAgent),
	})
}
