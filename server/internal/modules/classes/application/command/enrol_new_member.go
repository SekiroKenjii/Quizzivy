package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/content"
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
	code, ok := s.Lookup(cmd.Code)
	if !ok {
		return domain.EnrolResult{Outcome: domain.PreviewInvalid}, nil
	}
	member := cmd.Member
	member.FullName = content.NFCWithin(member.FullName, domain.MaxMemberName)
	result, err := s.Repo.Enrol(ctx, domain.EnrolInput{
		Code:      code,
		NewMember: &member,
		Now:       s.Now(),
		IP:        opt.String(cmd.Meta.IP),
		UserAgent: opt.String(cmd.Meta.UserAgent),
	})
	if err != nil {
		return result, err
	}
	s.Joined(ctx, result)
	return result, nil
}
