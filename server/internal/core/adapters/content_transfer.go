package adapters

import (
	"context"
	assignmentscmd "quizzivy/internal/modules/assignments/application/command"
	classescmd "quizzivy/internal/modules/classes/application/command"
	identitydomain "quizzivy/internal/modules/identity/domain"
	identityrepo "quizzivy/internal/modules/identity/repositories"
	mediacmd "quizzivy/internal/modules/media/application/command"
	questionscmd "quizzivy/internal/modules/questions/application/command"
	testscmd "quizzivy/internal/modules/tests/application/command"
	testsdomain "quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/cqrs"

	"github.com/jackc/pgx/v5"
)

// ContentMovers are the owning modules' transfers, each bound to the
// transaction a ContentTransfer opens.
type ContentMovers struct {
	Ledger      identitydomain.TransferLedger
	Classes     cqrs.CommandHandler[classescmd.TransferOwnership, int]
	Tests       cqrs.CommandHandler[testscmd.TransferOwnership, testsdomain.Transferred]
	Questions   cqrs.CommandHandler[questionscmd.TransferOwnership, int]
	Media       cqrs.CommandHandler[mediacmd.TransferOwnership, int]
	Assignments cqrs.CommandHandler[assignmentscmd.TransferOwnership, int]
}

// ContentTransfer implements the identity module's ContentMover: it locks both
// accounts through the ledger, moves classes, tests and question groups,
// questions, media and assignments in that order, and records the audit row,
// all in one transaction, so a failure at any step moves nothing.
type ContentTransfer struct {
	DB     db.Context
	Movers func(db.Context) ContentMovers
}

// Move implements the identity module's ContentMover.
func (c ContentTransfer) Move(ctx context.Context, req identitydomain.WriteRequest, in identitydomain.ContentTransfer) (identitydomain.TransferredContent, error) {
	var out identitydomain.TransferredContent
	err := c.DB.InTx(ctx, "transfer user content", func(tx pgx.Tx) error {
		m := c.Movers(db.NewContext(tx))
		if err := m.Ledger.LockTransfer(ctx, in); err != nil {
			return err
		}
		moved, err := m.move(ctx, in.FromID, in.ToID)
		if err != nil {
			return err
		}
		left, err := m.Ledger.RecordTransfer(ctx, req, in, moved)
		if err != nil {
			return err
		}
		out = identitydomain.TransferredContent{Moved: moved, LeftBehind: left}
		return nil
	})
	if err != nil {
		return identitydomain.TransferredContent{}, identityrepo.Contended(err)
	}
	return out, nil
}

func (m ContentMovers) move(ctx context.Context, from, to string) (identitydomain.MovedContent, error) {
	var moved identitydomain.MovedContent
	var err error
	if moved.Classes, err = m.Classes.Handle(ctx, classescmd.TransferOwnership{From: from, To: to}); err != nil {
		return moved, err
	}
	tests, err := m.Tests.Handle(ctx, testscmd.TransferOwnership{From: from, To: to})
	if err != nil {
		return moved, err
	}
	moved.Tests, moved.QuestionGroups = tests.Tests, tests.Groups
	if moved.Questions, err = m.Questions.Handle(ctx, questionscmd.TransferOwnership{From: from, To: to}); err != nil {
		return moved, err
	}
	if moved.Media, err = m.Media.Handle(ctx, mediacmd.TransferOwnership{From: from, To: to}); err != nil {
		return moved, err
	}
	moved.Assignments, err = m.Assignments.Handle(ctx, assignmentscmd.TransferOwnership{From: from, To: to})
	return moved, err
}
