package command

import (
	"context"
	"quizzivy/internal/modules/questions/application/internal/support"
	"quizzivy/internal/modules/questions/domain"
)

// Duplicate is A-06a's "Nhân bản": the same question again as a new bank row
// -- options, blanks, media and tags copied, ids fresh -- that no test holds
// yet. It goes through the same write as a create, so it is validated and
// audited like one.
type Duplicate struct {
	Request domain.WriteRequest
}

type DuplicateHandler struct {
	*support.Service
}

func (s DuplicateHandler) Handle(ctx context.Context, cmd Duplicate) (domain.Question, error) {
	source, err := s.Repo.Get(ctx, cmd.Request.ID)
	if err != nil {
		return domain.Question{}, err
	}
	cmd.Request.ID = ""
	cmd.Request.Input = support.InputOf(source)
	return s.Write(ctx, cmd.Request, false)
}
