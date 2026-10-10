package wiring

import (
	"log/slog"

	classesapp "quizzivy/internal/modules/classes/application"
	classesports "quizzivy/internal/modules/classes/application/ports"
	classesdomain "quizzivy/internal/modules/classes/domain"
	classeshttp "quizzivy/internal/modules/classes/http"
	classesrepo "quizzivy/internal/modules/classes/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/stats"
)

func classes(dbx db.Context, stats stats.Source, keys classesdomain.JoinCodeKeys, notifier classesports.Notifier, logger *slog.Logger) *classesapp.Application {
	return classesapp.New(classesrepo.NewPostgres(dbx), stats, keys).WithNotifier(notifier).WithLogger(logger)
}

func classesTransport(app *classesapp.Application) classeshttp.Classes {
	return classeshttp.NewClasses(app)
}
