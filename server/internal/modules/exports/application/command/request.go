package command

import (
	"context"

	"quizzivy/internal/modules/exports/application/internal/support"
	"quizzivy/internal/modules/exports/domain"
	"quizzivy/internal/shared/actor"
)

// Request queues an export of the centre's data for Actor, with the answers of
// every attempt when IncludeAnswers is true, audits it, and wakes the worker.
// It fails with a *domain.TooSoonError while Actor has an export that has not
// failed from the last hour, and with domain.ErrInProgress while any other
// export is queued or running.
type Request struct {
	Actor          actor.Actor
	IncludeAnswers bool
}

type RequestHandler struct {
	*support.Service
}

func (h RequestHandler) Handle(ctx context.Context, c Request) (domain.Export, error) {
	queued, err := h.Repo.Request(ctx, domain.Request{
		ActorID: c.Actor.ID, IP: c.Actor.IP, UserAgent: c.Actor.UserAgent, IncludeAnswers: c.IncludeAnswers,
	})
	if err != nil {
		return domain.Export{}, err
	}
	h.Wake.Wake()
	return queued, nil
}
