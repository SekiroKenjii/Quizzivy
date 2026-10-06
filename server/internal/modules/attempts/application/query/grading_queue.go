package query

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

// GradingQueue selects pending manual answers without changing their papers.
type GradingQueue = domain.GradingQueueQuery

// GradingQueueHandler enriches the authorized queue with frozen teaching context.
type GradingQueueHandler struct{ *support.Review }

func (h GradingQueueHandler) Handle(ctx context.Context, q GradingQueue) (domain.GradingQueue, error) {
	if q.Mode == "" {
		q.Mode = "student"
	}
	out, err := h.Repo.GradingQueue(ctx, q)
	if err != nil {
		return domain.GradingQueue{}, err
	}
	return h.QueueContext(ctx, out)
}
