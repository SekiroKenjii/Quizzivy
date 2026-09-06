package http

import (
	"context"
	"quizzivy/internal/modules/tests/application"

	mediadomain "quizzivy/internal/modules/media/domain"
)

// Media resolves a preview question's attachment; nil when object storage is off.
type Media interface {
	Get(ctx context.Context, id string) (mediadomain.Asset, error)
	SignedURL(ctx context.Context, asset mediadomain.Asset) (string, error)
}

type Tests struct {
	app   *application.Application
	media Media
}

func NewTests(app *application.Application, media Media) Tests {
	return Tests{app: app, media: media}
}
