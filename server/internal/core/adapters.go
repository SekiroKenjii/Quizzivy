package core

import (
	"context"
	"errors"
	"io"
	mediaquery "quizzivy/internal/modules/media/application/query"

	attemptshttp "quizzivy/internal/modules/attempts/http"
	classesapp "quizzivy/internal/modules/classes/application"
	classescommand "quizzivy/internal/modules/classes/application/command"
	classesdomain "quizzivy/internal/modules/classes/domain"
	identityapp "quizzivy/internal/modules/identity/application"
	identitydomain "quizzivy/internal/modules/identity/domain"
	mediaapp "quizzivy/internal/modules/media/application"
	mediamodel "quizzivy/internal/modules/media/application/model"
	mediadomain "quizzivy/internal/modules/media/domain"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	questionshttp "quizzivy/internal/modules/questions/http"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/platform/google"
	"quizzivy/internal/platform/probe"
	"time"
)

type googleProvider struct{ provider *google.Provider }

func (g googleProvider) Exchange(ctx context.Context, code, codeVerifier, redirectURI string) (string, error) {
	token, err := g.provider.Exchange(ctx, code, codeVerifier, redirectURI)
	return token, googleError(err)
}

func (g googleProvider) Verify(ctx context.Context, rawIDToken string) (identityapp.GoogleIdentity, error) {
	id, err := g.provider.Verify(ctx, rawIDToken)
	if err != nil {
		return identityapp.GoogleIdentity{}, googleError(err)
	}
	return identityapp.GoogleIdentity{
		Subject: id.Subject, Email: id.Email, EmailVerified: id.EmailVerified, Name: id.Name, Picture: id.Picture,
	}, nil
}

func googleError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, google.ErrExchangeFailed):
		return errors.Join(identitydomain.ErrGoogleExchangeFailed, err)
	case errors.Is(err, google.ErrRedirectNotAllowed):
		return errors.Join(identitydomain.ErrGoogleRedirectNotAllowed, err)
	case errors.Is(err, google.ErrTokenInvalid):
		return errors.Join(identitydomain.ErrGoogleTokenInvalid, err)
	case errors.Is(err, google.ErrEmailUnverified):
		return errors.Join(identitydomain.ErrGoogleEmailUnverified, err)
	}
	return err
}

type selfEnroller struct{ app *classesapp.Application }

func (e selfEnroller) EnrolNewMember(ctx context.Context, m classesdomain.NewMember, rawCode string, meta classesdomain.Meta) (classesdomain.EnrolResult, error) {
	return e.app.Commands.EnrolNewMember.Handle(ctx, classescommand.EnrolNewMember{Member: m, Code: rawCode, Meta: meta})
}

type audioProbe struct{}

func (audioProbe) Audio(r io.ReaderAt, size int64) (string, int, error) {
	mime, durationMs, err := probe.Audio(r, size)
	switch {
	case errors.Is(err, probe.ErrUnsupportedType):
		return "", 0, errors.Join(mediadomain.ErrUnsupportedType, err)
	case errors.Is(err, probe.ErrUnmeasurable):
		return "", 0, errors.Join(mediadomain.ErrUnmeasurable, err)
	}
	return mime, durationMs, err
}

type mediaKinds struct{ media *mediaapp.Application }

func (m mediaKinds) Kind(ctx context.Context, assetID string) (string, error) {
	if m.media == nil {
		return "", questionsdomain.ErrMediaNotFound
	}
	asset, err := m.media.Queries.Get.Handle(ctx, mediaquery.Get{ID: assetID})
	if errors.Is(err, mediadomain.ErrNotFound) {
		return "", questionsdomain.ErrMediaNotFound
	}
	if err != nil {
		return "", err
	}
	return string(asset.Kind), nil
}

type mediaPort struct{ app *mediaapp.Application }

func (p mediaPort) Get(ctx context.Context, id string) (mediadomain.Asset, error) {
	return p.app.Queries.Get.Handle(ctx, mediaquery.Get{ID: id})
}

func (p mediaPort) SignedURL(ctx context.Context, asset mediadomain.Asset) (string, error) {
	return p.app.Queries.SignedURL.Handle(ctx, mediaquery.SignedURL{Asset: asset})
}

func (p mediaPort) MintForStudent(ctx context.Context, studentID, assetID string) (mediamodel.SignedURLResult, error) {
	return p.app.Queries.MintForStudent.Handle(ctx, mediaquery.MintForStudent{StudentID: studentID, AssetID: assetID})
}

func (p mediaPort) SignedURLTTL() time.Duration {
	return p.app.SignedURLTTL()
}

func questionsMedia(app *mediaapp.Application) questionshttp.Media {
	if app == nil {
		return nil
	}
	return mediaPort{app}
}

func testsMedia(app *mediaapp.Application) testshttp.Media {
	if app == nil {
		return nil
	}
	return mediaPort{app}
}

func attemptsMedia(app *mediaapp.Application) attemptshttp.Media {
	if app == nil {
		return nil
	}
	return mediaPort{app}
}
