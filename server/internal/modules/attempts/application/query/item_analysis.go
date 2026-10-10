package query

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
)

// ItemAnalysis reads how the students Scope reaches did on each question of an
// assignment Scope reaches.
type ItemAnalysis struct {
	AssignmentID string
	Scope        access.Scope
}

// ItemAnalysisHandler closes the attempts that ran out of time before it reads,
// as the monitor does, so a student whose time is up is among the papers.
type ItemAnalysisHandler struct {
	*support.Service
}

func (h ItemAnalysisHandler) Handle(ctx context.Context, q ItemAnalysis) (domain.ItemAnalysis, error) {
	if err := h.ExpireDue(ctx, q.Scope, q.AssignmentID); err != nil {
		return domain.ItemAnalysis{}, err
	}
	return h.Store.ItemAnalysis(ctx, q.Scope, q.AssignmentID)
}
