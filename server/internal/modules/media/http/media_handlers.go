package http

import (
	"quizzivy/internal/modules/media/application"
)

type Media struct {
	app *application.Application
}

func NewMedia(app *application.Application) Media {
	return Media{app: app}
}
