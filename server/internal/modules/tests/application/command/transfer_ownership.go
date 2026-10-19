package command

import (
	"context"
	"quizzivy/internal/modules/tests/domain"
)

// TransferOwnership gives every test and question group From owns to To.
type TransferOwnership struct {
	From string
	To   string
}

// TransferOwnershipHandler moves tests and question groups through Repo, which
// is bound to the transaction the whole transfer runs in. The questions the
// tests use move in the same transaction through the questions module, so a
// draft still uses only its owner's questions.
type TransferOwnershipHandler struct {
	Repo domain.OwnershipTransfer
}

// Handle returns what moved, as domain.OwnershipTransfer counts it.
func (h TransferOwnershipHandler) Handle(ctx context.Context, cmd TransferOwnership) (domain.Transferred, error) {
	return h.Repo.TransferOwnership(ctx, cmd.From, cmd.To)
}
