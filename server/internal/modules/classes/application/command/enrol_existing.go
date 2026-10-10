package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/opt"
)

// EnrolExisting enrols a student who is already signed in (§6.2).
type EnrolExisting struct {
	UserID string
	Code   string
	Meta   domain.Meta
}

type EnrolExistingHandler struct {
	*support.Enrolment
}

func (s EnrolExistingHandler) Handle(ctx context.Context, cmd EnrolExisting) (domain.EnrolResult, error) {
	code, ok := s.Lookup(cmd.Code)
	if !ok {
		return domain.EnrolResult{Outcome: domain.PreviewInvalid}, nil
	}
	result, err := s.Repo.Enrol(ctx, domain.EnrolInput{
		Code:           code,
		ExistingUserID: cmd.UserID,
		Now:            s.Now(),
		IP:             opt.String(cmd.Meta.IP),
		UserAgent:      opt.String(cmd.Meta.UserAgent),
	})
	if err != nil {
		return result, err
	}
	s.Joined(ctx, result)
	return result, nil
}
