package command

import (
	"context"
	"strings"
	"unicode/utf8"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/validation"
)

// RequestCredits asks the Admins for more credits in the current month. The amount is between domain.MinRequest and domain.MaxRequest and the reason, trimmed, between one and domain.MaxReasonLength characters; a refusal carries every field at once. It answers domain.ErrRequestsOff while the centre's rule is to pause a teacher who runs out, and domain.ErrRequestPending while the teacher's earlier request this month is undecided. Every user who manages credits is told once it is recorded.
type RequestCredits struct {
	Actor  actor.Actor
	Amount int64
	Reason string
}

type RequestCreditsHandler struct {
	*support.Service
}

func (h RequestCreditsHandler) Handle(ctx context.Context, cmd RequestCredits) (domain.Request, error) {
	reason := strings.TrimSpace(cmd.Reason)
	var refused validation.Error
	if cmd.Amount < domain.MinRequest || cmd.Amount > domain.MaxRequest {
		refused.Add("amount", "outside the amount a request may ask for")
	}
	if n := utf8.RuneCountInString(reason); n < 1 || n > domain.MaxReasonLength {
		refused.Add("reason", "must be between 1 and 500 characters")
	}
	if err := refused.OrNil(); err != nil {
		return domain.Request{}, err
	}
	filed, err := h.Repo.CreateRequest(ctx, domain.CreateRequestInput{
		Month: h.Month(), UserID: cmd.Actor.ID, Amount: cmd.Amount, Reason: reason,
	})
	if err != nil {
		return domain.Request{}, err
	}
	h.TellManagers(ctx, filed)
	return filed.Request, nil
}
