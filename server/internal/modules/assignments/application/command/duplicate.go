package command

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
	"time"
)

// Duplicate copies the assignment Request names as a draft assigned to
// ClassIDs.
type Duplicate struct {
	Request  domain.Request
	ClassIDs []string
	Now      time.Time
}

type DuplicateHandler struct {
	*support.Service
}

func (s DuplicateHandler) Handle(ctx context.Context, cmd Duplicate) (domain.Assignment, error) {
	return s.Repo.Duplicate(ctx, cmd.Request, cmd.ClassIDs, cmd.Now)
}
