package command

import (
	"context"
	"quizzivy/internal/modules/questions/domain"
)

// TransferOwnership gives every question From owns to To.
type TransferOwnership struct {
	From string
	To   string
}

// TransferOwnershipHandler moves questions through Repo, which is bound to the
// transaction the whole transfer runs in.
type TransferOwnershipHandler struct {
	Repo domain.OwnershipTransfer
}

// Handle returns how many questions moved, as domain.OwnershipTransfer counts them.
func (h TransferOwnershipHandler) Handle(ctx context.Context, cmd TransferOwnership) (int, error) {
	return h.Repo.TransferOwnership(ctx, cmd.From, cmd.To)
}
