package http

import (
	"context"
	"log/slog"
	"quizzivy/internal/modules/attempts/application"
	identityquery "quizzivy/internal/modules/identity/application/query"
	mediamodel "quizzivy/internal/modules/media/application/model"
	"quizzivy/internal/shared/cqrs"
	"time"

	identitydomain "quizzivy/internal/modules/identity/domain"
	mediadomain "quizzivy/internal/modules/media/domain"
)

// Media resolves a question's audio; nil when object storage is off.
type Media interface {
	Get(ctx context.Context, id string) (mediadomain.Asset, error)
	SignedURL(ctx context.Context, asset mediadomain.Asset) (string, error)
	MintForStudent(ctx context.Context, studentID, assetID string) (mediamodel.SignedURLResult, error)
	SignedURLTTL() time.Duration
}

// Students names the student behind a paper for the teacher's review: the identity module's GetStudent query.
type Students = cqrs.QueryHandler[identityquery.GetStudent, identitydomain.Student]

type Attempts struct {
	app      *application.Application
	media    Media
	students Students
	logger   *slog.Logger
}

func NewAttempts(app *application.Application, media Media, students Students, logger *slog.Logger) Attempts {
	return Attempts{app: app, media: media, students: students, logger: logger}
}

func (h Attempts) log() *slog.Logger {
	if h.logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return h.logger
}
