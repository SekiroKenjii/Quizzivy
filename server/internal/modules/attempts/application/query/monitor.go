package query

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
)

// Monitor reads G-02 for an assignment Scope reaches.
type Monitor struct {
	AssignmentID string
	Scope        access.Scope
}

type MonitorHandler struct {
	*support.Service
}

func (s MonitorHandler) Handle(ctx context.Context, q Monitor) (domain.Monitor, error) {
	return s.Store.Monitor(ctx, q.Scope, q.AssignmentID, s.Now())
}
