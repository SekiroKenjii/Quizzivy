package query

import (
	"context"

	"quizzivy/internal/modules/organization/application/internal/support"
	"quizzivy/internal/modules/organization/application/model"
	"quizzivy/internal/modules/organization/domain"
)

// Settings asks for the stored document, read from the database.
type Settings struct{}

type SettingsHandler struct {
	*support.Service
}

func (h SettingsHandler) Handle(ctx context.Context, _ Settings) (domain.Settings, error) {
	return h.Repo.Read(ctx)
}

// Current asks for the document as this server remembers it, at most
// support.RefreshEvery old: what the front door shows.
type Current struct{}

type CurrentHandler struct {
	*support.Service
}

func (h CurrentHandler) Handle(ctx context.Context, _ Current) (domain.Settings, error) {
	return h.Snapshot.Settings(ctx)
}

// SignInPolicy asks for the sign-in policy in force, as Current reads it.
type SignInPolicy struct{}

type SignInPolicyHandler struct {
	*support.Service
}

func (h SignInPolicyHandler) Handle(ctx context.Context, _ SignInPolicy) (model.SignInPolicy, error) {
	s, err := h.Snapshot.Settings(ctx)
	if err != nil {
		return model.SignInPolicy{}, err
	}
	return model.SignInPolicy{
		Google: s.SignIn.Google, Password: s.SignIn.Password, ForcePasswordChange: s.SignIn.ForcePasswordChange,
		JoinCodes: s.SignIn.JoinCodes, AllowedGoogleDomains: s.SignIn.AllowedGoogleDomains,
	}, nil
}

// Branding asks for the centre's name and contact lines, as Current reads them.
type Branding struct{}

type BrandingHandler struct {
	*support.Service
}

func (h BrandingHandler) Handle(ctx context.Context, _ Branding) (model.Branding, error) {
	s, err := h.Snapshot.Settings(ctx)
	if err != nil {
		return model.Branding{}, err
	}
	p := s.Profile
	return model.Branding{
		Name: p.Name, ShortName: p.ShortName, Website: p.Website, ContactEmail: p.ContactEmail,
		FrontDeskPhone: p.FrontDeskPhone, FrontDeskHours: p.FrontDeskHours, Address: p.Address,
	}, nil
}
