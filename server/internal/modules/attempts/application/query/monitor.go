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
	return s.Store.Monitor(ctx, q.AssignmentID, s.Now())
}
