package command

import (
	"context"
	"quizzivy/internal/modules/classes/domain"
)

// TransferOwnership gives every class From teaches to To.
type TransferOwnership struct {
	From string
	To   string
}

// TransferOwnershipHandler moves classes through Repo, which is bound to the
// transaction the whole transfer runs in.
type TransferOwnershipHandler struct {
	Repo domain.OwnershipTransfer
}

// Handle returns how many classes moved, as domain.OwnershipTransfer counts them.
func (h TransferOwnershipHandler) Handle(ctx context.Context, cmd TransferOwnership) (int, error) {
	return h.Repo.TransferOwnership(ctx, cmd.From, cmd.To)
}
