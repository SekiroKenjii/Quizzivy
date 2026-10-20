package domain

import "context"

// OwnershipTransfer gives every assignment one account made to another.
type OwnershipTransfer interface {
	// TransferOwnership moves the assignments of from to to, deleted ones too, locking
	// them in id order, and returns how many moved, counting every one.
	TransferOwnership(ctx context.Context, from, to string) (int, error)
}
