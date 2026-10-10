package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

// Save is the service's side: it owns the clock, and nothing else here needs
// deciding.
type Save struct {
	Input domain.SaveInput
}

type SaveHandler struct {
	*support.Service
}

func (s SaveHandler) Handle(ctx context.Context, cmd Save) (domain.SaveResult, error) {
	if cmd.Input.Answers != nil {
		answers := make([]domain.Answer, len(cmd.Input.Answers))
		for i, answer := range cmd.Input.Answers {
			answers[i] = answer.Composed()
		}
		cmd.Input.Answers = answers
	}
	saved, reached, err := s.Store.Save(ctx, cmd.Input, s.Now())
	if err != nil {
		return saved, err
	}
	s.Announcer.Announce(ctx, reached)
	return saved, nil
}
