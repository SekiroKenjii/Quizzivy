package query

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

// CheckSetPasswordLink backs POST /auth/set-password/check: what a link is
// for, so the page can name the account before it asks for a password.
type CheckSetPasswordLink struct {
	Token string
}

type CheckSetPasswordLinkHandler struct {
	*support.Service
}

// Handle fails with domain.ErrSetPasswordLinkInvalid for a link that cannot be
// used, whatever the reason. It changes nothing.
func (s CheckSetPasswordLinkHandler) Handle(ctx context.Context, q CheckSetPasswordLink) (domain.SetPasswordLink, error) {
	if s.Links == nil {
		return domain.SetPasswordLink{}, domain.ErrSetPasswordLinkInvalid
	}
	link, err := s.Links.Find(ctx, domain.HashSetPasswordToken(q.Token))
	if err != nil {
		return domain.SetPasswordLink{}, err
	}
	if !link.Usable(s.Now()) {
		return domain.SetPasswordLink{}, domain.ErrSetPasswordLinkInvalid
	}
	return link, nil
}
