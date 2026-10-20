package domain

import "context"

// OwnershipTransfer gives every question one account owns to another.
type OwnershipTransfer interface {
	// TransferOwnership moves the questions of from to to, deleted ones too, locking
	// them in id order, and returns how many moved, counting those not deleted.
	TransferOwnership(ctx context.Context, from, to string) (int, error)
}
