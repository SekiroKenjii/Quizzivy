package wiring

import (
	"quizzivy/internal/core/adapters"
	assignmentscmd "quizzivy/internal/modules/assignments/application/command"
	assignmentsrepo "quizzivy/internal/modules/assignments/repositories"
	classescmd "quizzivy/internal/modules/classes/application/command"
	classesrepo "quizzivy/internal/modules/classes/repositories"
	identityapp "quizzivy/internal/modules/identity/application"
	identityports "quizzivy/internal/modules/identity/application/ports"
	identityrepo "quizzivy/internal/modules/identity/repositories"
	mediacmd "quizzivy/internal/modules/media/application/command"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionscmd "quizzivy/internal/modules/questions/application/command"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	testscmd "quizzivy/internal/modules/tests/application/command"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
)

func contentTransfer(dbx db.Context, app *identityapp.Application, notifier identityports.Notifier) {
	app.SetContentTransfer(adapters.ContentTransfer{
		DB: dbx,
		Movers: func(scoped db.Context) adapters.ContentMovers {
			return adapters.ContentMovers{
				Ledger:      identityrepo.NewTransferLedger(scoped),
				Classes:     classescmd.TransferOwnershipHandler{Repo: classesrepo.NewPostgres(scoped)},
				Tests:       testscmd.TransferOwnershipHandler{Repo: testsrepo.NewPostgres(scoped, nil, nil)},
				Questions:   questionscmd.TransferOwnershipHandler{Repo: questionsrepo.NewPostgres(scoped)},
				Media:       mediacmd.TransferOwnershipHandler{Repo: mediarepo.NewPostgres(scoped)},
				Assignments: assignmentscmd.TransferOwnershipHandler{Repo: assignmentsrepo.NewPostgres(scoped)},
			}
		},
	}, notifier)
}
