package query

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
)

// Monitor reads the live roster of an assignment Scope reaches: one row per
// targeted student the scope reaches.
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
