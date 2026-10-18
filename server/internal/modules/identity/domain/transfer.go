package domain

import (
	"context"
	"errors"
	"time"
)

// ErrUserNotDisabled means content was to be moved away from an account that
// is not disabled. Nothing was moved.
var ErrUserNotDisabled = errors.New("users: only a disabled account's content can be transferred")

// ErrInvalidRecipient means the account content was to be moved to does not
// exist, is disabled, holds no teacher workspace or is the source itself.
// Nothing was moved.
var ErrInvalidRecipient = errors.New("users: the recipient cannot take this account's content")

// ContentTransfer moves what FromID owns to ToID. FromRoleID is the role the
// source held when the caller's right to act on it was checked.
type ContentTransfer struct {
	FromID     string
	FromRoleID string
	ToID       string
	Now        time.Time
}

// MovedContent counts what a transfer gave the recipient, as OwnedContent
// counts it, with question groups beside it: every class and assignment, and
// the tests, questions, question groups and media not deleted.
type MovedContent struct {
	Classes        int
	Tests          int
	Questions      int
	QuestionGroups int
	Media          int
	Assignments    int
}

// Total is how many things moved.
func (m MovedContent) Total() int {
	return m.Classes + m.Tests + m.Questions + m.QuestionGroups + m.Media + m.Assignments
}

// LeftBehind counts what a transfer leaves with the source: its Word imports
// that are not finished, and the student-like accounts it created that the
// recipient does not reach after the move.
type LeftBehind struct {
	Imports  int
	Students int
}

// TransferredContent is what a transfer moved and what it left behind.
type TransferredContent struct {
	Moved      MovedContent
	LeftBehind LeftBehind
}

// TransferLedger is the accounts' side of a transfer, bound to the
// transaction the content moves in.
type TransferLedger interface {
	// LockTransfer locks the source and the recipient FOR NO KEY UPDATE in id
	// order and then the recipient's role FOR SHARE, so neither account nor the
	// role changes before the commit. It fails with ErrUserNotFound for a
	// missing source, ErrRoleChanged when the source no longer holds
	// FromRoleID, ErrUserNotDisabled and ErrInvalidRecipient.
	LockTransfer(ctx context.Context, in ContentTransfer) error
	// RecordTransfer counts, after the moves, what stays with the source and
	// writes the user.content_transferred audit row with the owner change as
	// both names, the recipient and the counts.
	RecordTransfer(ctx context.Context, req WriteRequest, in ContentTransfer, moved MovedContent) (LeftBehind, error)
}
