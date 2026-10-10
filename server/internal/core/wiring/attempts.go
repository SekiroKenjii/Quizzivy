package wiring

import (
	"log/slog"

	"quizzivy/internal/core/adapters"
	attemptsapp "quizzivy/internal/modules/attempts/application"
	attemptsports "quizzivy/internal/modules/attempts/application/ports"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	identityapp "quizzivy/internal/modules/identity/application"
	mediaapp "quizzivy/internal/modules/media/application"
	"quizzivy/internal/platform/db"
)

func attempts(dbx db.Context, notifier attemptsports.Notifier, logger *slog.Logger) *attemptsapp.Application {
	return attemptsapp.New(attemptsrepo.NewTimelines(dbx), attemptsrepo.NewReviews(dbx), attemptsrepo.NewPostgres(dbx, adapters.AttemptStartGuard{})).WithNotifier(notifier, logger)
}

func attemptsTransport(app *attemptsapp.Application, media *mediaapp.Application, identity *identityapp.Application, logger *slog.Logger) attemptshttp.Attempts {
	return attemptshttp.NewAttempts(app, adapters.AttemptsMedia(media), identity.Queries.StudentAccount, logger)
}
