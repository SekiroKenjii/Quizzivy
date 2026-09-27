package wiring

import (
	"log/slog"

	availabilityapp "quizzivy/internal/modules/availability/application"
	availabilityhttp "quizzivy/internal/modules/availability/http"
	availabilityrepo "quizzivy/internal/modules/availability/repositories"
	"quizzivy/internal/platform/db"
)

func availability(dbx db.Context, logger *slog.Logger) (*availabilityapp.Application, availabilityhttp.Availability) {
	app := availabilityapp.New(availabilityrepo.NewPostgres(dbx), logger)
	return app, availabilityhttp.NewAvailability(app)
}
