package command

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/domain"
)

// Settle ends a reservation with what the provider metered: it prices Usage at the tier's configured prices, moves that many credits from reserved to used on the pool and the teacher, appends the ledger row and deletes the reservation. The charge never exceeds the reservation, so a call the provider has been paid for is not refused by a constraint; the ledger keeps the real token counts, and the log notes a call that cost more than its ceiling. It answers domain.ErrReservationGone when the reservation is no longer held. The first settlement that takes a teacher past the warning share tells them, once a month.
type Settle struct {
	Reservation domain.Reservation
	Tier        ports.Tier
	Usage       ports.Usage
	RequestID   string
}

type SettleHandler struct {
	*support.Service
}

func (h SettleHandler) Handle(ctx context.Context, cmd Settle) (domain.Settled, error) {
	price, err := h.Prices.For(domain.Tier(cmd.Tier))
	if err != nil {
		return domain.Settled{}, err
	}
	tokens := support.TokensOf(cmd.Usage)
	metered, err := price.Credits(tokens)
	if err != nil {
		return domain.Settled{}, err
	}
	settled, err := h.Repo.Settle(ctx, domain.Settlement{
		Reservation: cmd.Reservation, Tier: domain.Tier(cmd.Tier), Tokens: tokens, Metered: metered, RequestID: cmd.RequestID,
	})
	if err != nil {
		return domain.Settled{}, err
	}
	if settled.Charged < settled.Metered {
		h.Logger.WarnContext(ctx, "ai call cost more than its ceiling",
			"feature", cmd.Reservation.Feature, "ceiling", cmd.Reservation.Credits, "metered", settled.Metered, "request_id", cmd.RequestID)
	}
	if settled.Warning != nil {
		h.TellWarning(ctx, *settled.Warning)
	}
	return settled, nil
}
