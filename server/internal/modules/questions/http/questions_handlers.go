package http

import (
	"context"
	"quizzivy/internal/modules/questions/application"

	mediadomain "quizzivy/internal/modules/media/domain"
)

// Media resolves a question's attachment for rendering; nil when object storage is off.
type Media interface {
	Get(ctx context.Context, id string) (mediadomain.Asset, error)
	SignedURL(ctx context.Context, asset mediadomain.Asset) (string, error)
}

type Questions struct {
	app   *application.Application
	media Media
}

func NewQuestions(app *application.Application, media Media) Questions {
	return Questions{app: app, media: media}
}
