package query

import (
	"bytes"
	"context"

	"quizzivy/internal/modules/organization/application/internal/support"
	"quizzivy/internal/modules/organization/domain"
)

// Logo asks for the bytes of the centre's logo of Variant.
type Logo struct {
	Variant domain.Variant
}

// LogoBytes is a logo's bytes with the type and hash it is stored under.
type LogoBytes struct {
	Logo  domain.Logo
	Bytes []byte
}

// Current reports whether digest is the hash these bytes are stored under.
func (b LogoBytes) Current(digest []byte) bool { return bytes.Equal(b.Logo.SHA256, digest) }

type LogoHandler struct {
	*support.Service
}

// Handle answers domain.ErrNoLogo when the centre has no logo of the variant,
// and domain.ErrStorageOff when this server cannot read one.
func (h LogoHandler) Handle(ctx context.Context, q Logo) (LogoBytes, error) {
	s, err := h.Snapshot.Settings(ctx)
	if err != nil {
		return LogoBytes{}, err
	}
	logo := s.Profile.Logo(q.Variant)
	if logo == nil {
		return LogoBytes{}, domain.ErrNoLogo
	}
	data, err := h.LogoBytes(ctx, *logo, s.Profile.LightLogo, s.Profile.DarkLogo)
	if err != nil {
		return LogoBytes{}, err
	}
	return LogoBytes{Logo: *logo, Bytes: data}, nil
}
