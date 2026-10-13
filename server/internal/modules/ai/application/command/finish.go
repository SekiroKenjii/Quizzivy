package command

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/domain"
)

// Finish ends a reservation once the call it held has returned, with the Completer's response and error. The provider bills a completion it refused (ports.ErrRefused) and one whose answer was not a complete JSON object (ports.ErrInvalidResponse), and the Completer returns the usage it reported with both, so those settle like a success. Every other failure, an unavailable provider, a rejected request or a cancelled caller, billed nothing and releases. A response that reports no tokens releases whatever the error. The Settled it returns has a Charged of zero when the reservation was released. A reservation that covered several calls is ended once, with the Response of a Tally that holds the billed ones and a nil Err: the tally has already left out the calls that billed nothing, so the sum settles when anything was billed and releases when nothing was.
type Finish struct {
	Reservation domain.Reservation
	Tier        ports.Tier
	Response    ports.Response
	Err         error
}

type FinishHandler struct {
	*support.Service
}

func (h FinishHandler) Handle(ctx context.Context, cmd Finish) (domain.Settled, error) {
	if !Billed(cmd.Err) || support.TokensOf(cmd.Response.Usage).Zero() {
		_, err := h.Repo.Release(ctx, cmd.Reservation)
		return domain.Settled{}, err
	}
	return SettleHandler(h).Handle(ctx, Settle{
		Reservation: cmd.Reservation, Tier: cmd.Tier, Usage: cmd.Response.Usage, RequestID: cmd.Response.RequestID,
	})
}
