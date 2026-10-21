package domain

import "context"

// Transferred counts what an ownership transfer moved: the tests and the
// question groups not deleted or archived.
type Transferred struct {
	Tests  int
	Groups int
}

// OwnershipTransfer gives every test and question group one account owns to
// another.
type OwnershipTransfer interface {
	// TransferOwnership moves the tests of from to to and then its question
	// groups, deleted and archived ones too, locking each table's rows in id
	// order.
	TransferOwnership(ctx context.Context, from, to string) (Transferred, error)
}
