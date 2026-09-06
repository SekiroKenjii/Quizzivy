package query

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

type Monitor struct {
	AssignmentID string
}

type MonitorHandler struct {
	*support.Service
}

func (s MonitorHandler) Handle(ctx context.Context, q Monitor) (domain.Monitor, error) {
	if err := s.ExpireDue(ctx, q.AssignmentID); err != nil {
		return domain.Monitor{}, err
	}
	return s.Store.Monitor(ctx, q.AssignmentID, s.Now())
}
