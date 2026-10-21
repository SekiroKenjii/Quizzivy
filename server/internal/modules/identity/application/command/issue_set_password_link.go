package command

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"time"
)

// IssueSetPasswordLink issues an account a link to set its password and
// enqueues the message that carries it, on Tx, the caller's transaction, so
// that the account, the link and the message commit together or not at all. It
// is a command for other commands to call, never an HTTP operation.
//
// An invite carries an account's first password. A reset ends the account's
// sessions, as every password reset does, unless KeepSessions says the person
// asking is the account's owner, signed in on purpose. By is the user who asked
// for the link, empty when a job did.
type IssueSetPasswordLink struct {
	Tx           domain.LinkTx
	UserID       string
	Purpose      domain.SetPasswordPurpose
	By           string
	KeepSessions bool
}

type IssueSetPasswordLinkHandler struct {
	*support.Service
}

// Handle fails with domain.ErrPasswordSignInDisabled for a reset while the
// centre has password sign-in off; an invite then sends a sign-in invite and no
// link. It fails with domain.ErrAccountDisabled for a disabled account and with
// domain.ErrSetPasswordUnavailable when this server cannot send a link.
func (s IssueSetPasswordLinkHandler) Handle(ctx context.Context, cmd IssueSetPasswordLink) (domain.IssuedLink, error) {
	if s.Links == nil {
		return domain.IssuedLink{}, domain.ErrSetPasswordUnavailable
	}
	if cmd.Tx == nil || !cmd.Purpose.Valid() {
		return domain.IssuedLink{}, errors.New("issue set-password link: a transaction and a purpose are required")
	}
	policy, err := s.SignInPolicy(ctx)
	if err != nil {
		return domain.IssuedLink{}, fmt.Errorf("read the sign-in policy: %w", err)
	}
	if !policy.Password && cmd.Purpose == domain.LinkReset {
		return domain.IssuedLink{}, domain.ErrPasswordSignInDisabled
	}
	recipient, err := cmd.Tx.Recipient(ctx, cmd.UserID)
	if err != nil {
		return domain.IssuedLink{}, err
	}
	if recipient.Disabled {
		return domain.IssuedLink{}, domain.ErrAccountDisabled
	}
	mail := domain.LinkMail{UserID: recipient.ID, To: recipient.Email, FullName: recipient.FullName}
	if !policy.Password {
		return s.signInInvite(ctx, cmd.Tx, mail)
	}
	return s.issue(ctx, cmd, recipient.ID, mail)
}

func (s IssueSetPasswordLinkHandler) signInInvite(ctx context.Context, tx domain.LinkTx, mail domain.LinkMail) (domain.IssuedLink, error) {
	mail.Kind = domain.MailSignInInvite
	if err := tx.Enqueue(ctx, mail); err != nil {
		return domain.IssuedLink{}, err
	}
	return domain.IssuedLink{SignInInvite: true}, nil
}

func (s IssueSetPasswordLinkHandler) issue(ctx context.Context, cmd IssueSetPasswordLink, userID string, mail domain.LinkMail) (domain.IssuedLink, error) {
	now := s.Now().Truncate(time.Microsecond)
	token, hash, err := domain.NewSetPasswordToken()
	if err != nil {
		return domain.IssuedLink{}, err
	}
	if err := cmd.Tx.RevokeUnused(ctx, userID, now); err != nil {
		return domain.IssuedLink{}, err
	}
	if err := cmd.Tx.Insert(ctx, domain.NewLink{UserID: userID, TokenHash: hash, Purpose: cmd.Purpose, CreatedBy: cmd.By, Now: now}); err != nil {
		return domain.IssuedLink{}, err
	}
	mail.Kind, mail.Token, mail.ExpiresAt = domain.MailAccountInvite, token, now.Add(domain.SetPasswordLifetime)
	if cmd.Purpose == domain.LinkReset {
		mail.Kind = domain.MailPasswordReset
		if !cmd.KeepSessions {
			if err := cmd.Tx.EndSessions(ctx, userID, now); err != nil {
				return domain.IssuedLink{}, err
			}
			cmd.Tx.AfterCommit(func() { s.Principals.Forget(userID) })
		}
	}
	if err := cmd.Tx.Enqueue(ctx, mail); err != nil {
		return domain.IssuedLink{}, err
	}
	return domain.IssuedLink{ExpiresAt: mail.ExpiresAt}, nil
}
