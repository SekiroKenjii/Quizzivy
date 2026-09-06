// Package adapters translates between the platform and the module ports: platform errors become domain errors, and one module's handlers become another module's port.
package adapters

import (
	"context"
	"errors"

	identitymodel "quizzivy/internal/modules/identity/application/model"
	identitydomain "quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/google"
)

// Google adapts the platform Google client to identity's GoogleProvider port.
type Google struct{ provider *google.Provider }

func NewGoogle(provider *google.Provider) Google {
	return Google{provider: provider}
}

func (g Google) Exchange(ctx context.Context, code, codeVerifier, redirectURI string) (string, error) {
	token, err := g.provider.Exchange(ctx, code, codeVerifier, redirectURI)
	return token, googleError(err)
}

func (g Google) Verify(ctx context.Context, rawIDToken string) (identitymodel.GoogleIdentity, error) {
	id, err := g.provider.Verify(ctx, rawIDToken)
	if err != nil {
		return identitymodel.GoogleIdentity{}, googleError(err)
	}
	return identitymodel.GoogleIdentity{
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
