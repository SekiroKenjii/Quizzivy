package command

import (
	"context"

	"github.com/google/uuid"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
)

// DecideRequest approves or declines a request, for a signed-in author who may act on the teacher it belongs to under the subset rule (access.CanActOn) and else domain.ErrForbidden; System is not asked. An approval raises the teacher's allowance for the month by the request's amount; a decline changes nothing but the status. Both tell the teacher and write one audit row naming By. It answers domain.ErrRequestNotFound for an id that is not a request, domain.ErrRequestDecided for one decided already and domain.ErrRequestExpired for an approval of an earlier month's request.
type DecideRequest struct {
	By      domain.Author
	ID      string
	Approve bool
}

type DecideRequestHandler struct {
	*support.Service
}

func (h DecideRequestHandler) Handle(ctx context.Context, cmd DecideRequest) (domain.Decided, error) {
	if err := checkAuthor(cmd.By); err != nil {
		return domain.Decided{}, err
	}
	if _, err := uuid.Parse(cmd.ID); err != nil {
		return domain.Decided{}, domain.ErrRequestNotFound
	}
	owner, err := h.Repo.RequestOwner(ctx, cmd.ID)
	if err != nil {
		return domain.Decided{}, err
	}
	if err := h.MayActOn(ctx, cmd.By, owner); err != nil {
		return domain.Decided{}, err
	}
	decided, err := h.Repo.DecideRequest(ctx, domain.DecideInput{By: cmd.By, ID: cmd.ID, Month: h.Month(), Approve: cmd.Approve})
	if err != nil {
		return domain.Decided{}, err
	}
	h.TellDecision(ctx, decided)
	return decided, nil
}
