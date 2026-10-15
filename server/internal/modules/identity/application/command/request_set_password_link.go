package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/opt"
)

// RequestSetPasswordLink emails a signed-in account a link to set a password:
// the way an account that signs in with Google gets one. It keeps the
// requester's sessions, since the requester is the account's owner and is
// signed in on purpose; the link, once used, ends the others.
type RequestSetPasswordLink struct {
	UserID    string
	IP        string
	UserAgent string
}

type RequestSetPasswordLinkHandler struct {
	*support.Service
}

// Handle fails with domain.ErrAccountDisabled for a disabled account, with
// domain.ErrPasswordSignInDisabled while the centre has password sign-in off,
// and with domain.ErrSetPasswordUnavailable when this server cannot send a link.
func (s RequestSetPasswordLinkHandler) Handle(ctx context.Context, cmd RequestSetPasswordLink) (cqrs.Nothing, error) {
	if s.Links == nil {
		return cqrs.Nothing{}, domain.ErrSetPasswordUnavailable
	}
	issue := IssueSetPasswordLinkHandler(s)
	return cqrs.Nothing{}, s.Links.InTx(ctx, func(ctx context.Context, tx domain.LinkTx) error {
		if _, err := issue.Handle(ctx, IssueSetPasswordLink{Tx: tx, UserID: cmd.UserID, Purpose: domain.LinkReset, By: cmd.UserID, KeepSessions: true}); err != nil {
			return err
		}
		return tx.Audit(ctx, audit.Entry{
			ActorUserID: &cmd.UserID,
			Action:      "user.password_link_requested",
			Entity:      "user",
			EntityID:    &cmd.UserID,
			OccurredAt:  s.Now(),
			IP:          opt.String(cmd.IP),
			UserAgent:   opt.String(cmd.UserAgent),
		})
	})
}
