package wiring

import (
	"quizzivy/internal/core/adapters"
	mediaapp "quizzivy/internal/modules/media/application"
	questionsapp "quizzivy/internal/modules/questions/application"
	questionshttp "quizzivy/internal/modules/questions/http"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/platform/db"
)

func questions(dbx db.Context, media *mediaapp.Application) (*questionsapp.Application, *questionsrepo.Postgres) {
	repo := questionsrepo.NewPostgres(dbx)
	return questionsapp.New(repo, adapters.MediaKinds{Media: media}), repo
}

func questionsTransport(app *questionsapp.Application, media *mediaapp.Application) questionshttp.Questions {
	return questionshttp.NewQuestions(app, adapters.QuestionsMedia(media))
}
