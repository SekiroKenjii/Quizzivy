package wiring

import (
	assignmentsapp "quizzivy/internal/modules/assignments/application"
	assignmentshttp "quizzivy/internal/modules/assignments/http"
	assignmentsrepo "quizzivy/internal/modules/assignments/repositories"
	"quizzivy/internal/platform/db"
)

func assignments(dbx db.Context) assignmentshttp.Assignments {
	return assignmentshttp.NewAssignments(assignmentsapp.New(assignmentsrepo.NewPostgres(dbx)))
}
