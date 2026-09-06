package wiring

import (
	classesapp "quizzivy/internal/modules/classes/application"
	classeshttp "quizzivy/internal/modules/classes/http"
	classesrepo "quizzivy/internal/modules/classes/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/stats"
)

func classes(dbx db.Context, stats stats.Source) *classesapp.Application {
	return classesapp.New(classesrepo.NewPostgres(dbx), stats)
}

func classesTransport(app *classesapp.Application) classeshttp.Classes {
	return classeshttp.NewClasses(app)
}
