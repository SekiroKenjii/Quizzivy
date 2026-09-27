package query

import (
	"context"

	"quizzivy/internal/modules/availability/application/internal/support"
	"quizzivy/internal/modules/availability/domain"
)

// CurrentWindow asks for the next maintenance window that has not ended.
type CurrentWindow struct{}

// CurrentWindowHandler answers CurrentWindow from the in-memory snapshot.
type CurrentWindowHandler struct {
	*support.Service
}

// Handle returns the status. It never fails: a failed read is the snapshot's to absorb.
func (h CurrentWindowHandler) Handle(ctx context.Context, _ CurrentWindow) (domain.Status, error) {
	return h.Snapshot.Status(ctx), nil
}
