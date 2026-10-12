package wiring

import (
	"log/slog"

	assignmentsapp "quizzivy/internal/modules/assignments/application"
	assignmentsports "quizzivy/internal/modules/assignments/application/ports"
	assignmentshttp "quizzivy/internal/modules/assignments/http"
	assignmentsrepo "quizzivy/internal/modules/assignments/repositories"
	"quizzivy/internal/platform/db"
)

func assignments(dbx db.Context, notifier assignmentsports.Notifier, logger *slog.Logger) assignmentshttp.Assignments {
	return assignmentshttp.NewAssignments(assignmentsapp.New(assignmentsrepo.NewPostgres(dbx)).WithNotifier(notifier, logger))
}
