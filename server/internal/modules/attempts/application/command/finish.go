package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
)

// Finish declares a paper on an assignment Scope reaches graded.
type Finish struct {
	AttemptID string
	Scope     access.Scope
}

type FinishHandler struct {
	*support.Review
}

func (r FinishHandler) Handle(ctx context.Context, cmd Finish) (domain.Attempt, error) {
	graded, err := r.Repo.Finish(ctx, cmd.Scope, cmd.AttemptID)
	if err != nil {
		return graded, err
	}
	r.Announcer.Announce(ctx, domain.Milestones{AttemptID: graded.ID, Graded: true})
	return graded, nil
}
