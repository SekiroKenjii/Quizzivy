package wiring

import (
	"quizzivy/internal/core/adapters"
	mediaapp "quizzivy/internal/modules/media/application"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	testsapp "quizzivy/internal/modules/tests/application"
	testshttp "quizzivy/internal/modules/tests/http"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
)

func tests(dbx db.Context, questions *questionsrepo.Postgres, media *mediarepo.Postgres, mediaApp *mediaapp.Application) *testsapp.Application {
	return testsapp.New(testsrepo.NewPostgres(dbx, questions, media).WithGroupQuestions(adapters.GroupQuestions{})).WithGroups(
		testsrepo.NewGroupsPostgres(dbx, adapters.GroupQuestions{}, media), adapters.MediaKinds{Media: mediaApp})
}

func testsTransport(app *testsapp.Application, media *mediaapp.Application) testshttp.Tests {
	return testshttp.NewTests(app, adapters.TestsMedia(media))
}
