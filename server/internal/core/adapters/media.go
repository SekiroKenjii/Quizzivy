package adapters

import (
	"context"
	"strings"
	"time"

	attemptshttp "quizzivy/internal/modules/attempts/http"
	mediaapp "quizzivy/internal/modules/media/application"
	mediamodel "quizzivy/internal/modules/media/application/model"
	mediaquery "quizzivy/internal/modules/media/application/query"
	mediadomain "quizzivy/internal/modules/media/domain"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	questionshttp "quizzivy/internal/modules/questions/http"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/shared/access"
)

// MediaKinds answers the questions and tests MediaKinds ports from the media
// application's Readable: the kind of an asset the scope may read, or
// ErrMediaNotFound alike for a missing, deleted or unreadable one. A nil
// application knows no asset.
type MediaKinds struct{ Media *mediaapp.Application }

func (m MediaKinds) Kind(ctx context.Context, scope access.Scope, assetID string) (string, error) {
	if m.Media == nil {
		return "", questionsdomain.ErrMediaNotFound
	}
	id := strings.ToLower(assetID)
	kinds, err := m.Media.Queries.Readable.Handle(ctx, mediaquery.Readable{Scope: scope, IDs: []string{id}})
	if err != nil {
		return "", err
	}
	kind, ok := kinds[id]
	if !ok {
		return "", questionsdomain.ErrMediaNotFound
	}
	return string(kind), nil
}

// Media serves the media port the questions, tests and attempts transports render attachments through.
type Media struct{ app *mediaapp.Application }

func (p Media) Get(ctx context.Context, id string) (mediadomain.Asset, error) {
	return p.app.Queries.Get.Handle(ctx, mediaquery.Get{ID: id})
}

func (p Media) SignedURL(ctx context.Context, asset mediadomain.Asset) (string, error) {
	return p.app.Queries.SignedURL.Handle(ctx, mediaquery.SignedURL{Asset: asset})
}

func (p Media) MintForStudent(ctx context.Context, studentID, assetID string) (mediamodel.SignedURLResult, error) {
	return p.app.Queries.MintForStudent.Handle(ctx, mediaquery.MintForStudent{StudentID: studentID, AssetID: assetID})
}

func (p Media) SignedURLTTL() time.Duration {
	return p.app.SignedURLTTL()
}

// QuestionsMedia is the media port for the questions transport, nil when object storage is off.
func QuestionsMedia(app *mediaapp.Application) questionshttp.Media {
	if app == nil {
		return nil
	}
	return Media{app: app}
}

// TestsMedia is the media port for the tests transport, nil when object storage is off.
func TestsMedia(app *mediaapp.Application) testshttp.Media {
	if app == nil {
		return nil
	}
	return Media{app: app}
}

// AttemptsMedia is the media port for the attempts transport, nil when object storage is off.
func AttemptsMedia(app *mediaapp.Application) attemptshttp.Media {
	if app == nil {
		return nil
	}
	return Media{app: app}
}
