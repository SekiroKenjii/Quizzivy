package wiring

import (
	"log/slog"

	"quizzivy/internal/core/adapters"
	classesapp "quizzivy/internal/modules/classes/application"
	classesports "quizzivy/internal/modules/classes/application/ports"
	classesdomain "quizzivy/internal/modules/classes/domain"
	classeshttp "quizzivy/internal/modules/classes/http"
	classesrepo "quizzivy/internal/modules/classes/repositories"
	identityapp "quizzivy/internal/modules/identity/application"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/stats"
)

func classes(dbx db.Context, stats stats.Source, keys classesdomain.JoinCodeKeys, notifier classesports.Notifier, logger *slog.Logger) *classesapp.Application {
	return classesapp.New(classesrepo.NewPostgres(dbx), stats, keys).WithNotifier(notifier).WithLogger(logger)
}

func attachClassPorts(app *classesapp.Application, identity *identityapp.Application, scores stats.ClassSource) {
	app.WithZones(adapters.ProfileZone{Query: identity.Queries.EffectiveZone}).
		WithAvatars(adapters.TeacherPhotos{Query: identity.Queries.AvatarURL}).
		WithClassScores(scores)
}

func classesTransport(app *classesapp.Application) classeshttp.Classes {
	return classeshttp.NewClasses(app)
}
