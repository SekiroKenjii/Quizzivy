package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/email"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Outbox is the email outbox a link's message is written to: the platform's
// Outbox, which writes inside the caller's transaction and wakes this
// process's dispatcher.
type Outbox interface {
	Enqueue(ctx context.Context, q db.Querier, m email.Mail) (string, bool, error)
	Notify()
}

// SetPasswordLinks persists set-password links and writes their messages to
// the outbox in the same transaction. A nil outbox cannot send a link.
type SetPasswordLinks struct {
	db.Repository
	outbox Outbox
}

// NewSetPasswordLinks returns the store over dbx that writes messages to outbox.
func NewSetPasswordLinks(dbx db.Context, outbox Outbox) *SetPasswordLinks {
	return &SetPasswordLinks{Repository: db.NewRepository(dbx), outbox: outbox}
}

// InTx runs fn in one transaction and, once it has committed, the functions fn
// registered with AfterCommit.
func (s *SetPasswordLinks) InTx(ctx context.Context, fn func(ctx context.Context, tx domain.LinkTx) error) error {
	var run func()
	err := s.Context.InTx(ctx, "identity: set-password link", func(tx pgx.Tx) error {
		var writes domain.LinkTx
		writes, run = s.Writes(tx)
		return fn(ctx, writes)
	})
	if err != nil {
		return err
	}
	run()
	return nil
}

// Writes returns the writes of issuing a link on tx, which the caller begins
// and commits, and the function that runs what they registered with
// AfterCommit. The caller calls it after its commit and never after a rollback.
func (s *SetPasswordLinks) Writes(tx pgx.Tx) (domain.LinkTx, func()) {
	w := &linkWrites{tx: tx, outbox: s.outbox}
	return w, w.runAfterCommit
}

type linkWrites struct {
	tx     pgx.Tx
	outbox Outbox
	after  []func()
	woken  bool
}

func (w *linkWrites) runAfterCommit() {
	for _, fn := range w.after {
		fn()
	}
	w.after = nil
}

func (w *linkWrites) AfterCommit(fn func()) { w.after = append(w.after, fn) }

func (w *linkWrites) Recipient(ctx context.Context, userID string) (domain.LinkRecipient, error) {
	var r domain.LinkRecipient
	err := w.tx.QueryRow(ctx, `
		SELECT id::text, email, full_name, disabled_at IS NOT NULL
		  FROM app.users
		 WHERE id = $1::uuid`, userID).Scan(&r.ID, &r.Email, &r.FullName, &r.Disabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.LinkRecipient{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.LinkRecipient{}, fmt.Errorf("read link recipient: %w", err)
	}
	return r, nil
}

func (w *linkWrites) RevokeUnused(ctx context.Context, userID string, at time.Time) error {
	if _, err := w.tx.Exec(ctx, revokeUnusedLinks, userID, at); err != nil {
		return fmt.Errorf("revoke unused set-password links: %w", err)
	}
	return nil
}

const revokeUnusedLinks = `
	UPDATE app.set_password_tokens
	   SET revoked_at = $2
	 WHERE user_id = $1::uuid AND used_at IS NULL AND revoked_at IS NULL`

func (w *linkWrites) Insert(ctx context.Context, link domain.NewLink) error {
	created := link.Now.Truncate(time.Microsecond)
	if _, err := w.tx.Exec(ctx, `
		INSERT INTO app.set_password_tokens (user_id, token_hash, purpose, created_by, created_at, expires_at)
		VALUES ($1::uuid, $2, $3, $4::uuid, $5, $6)`,
		link.UserID, link.TokenHash, string(link.Purpose), opt.String(link.CreatedBy),
		created, created.Add(domain.SetPasswordLifetime)); err != nil {
		return fmt.Errorf("store set-password link: %w", err)
	}
	return nil
}

func (w *linkWrites) EndSessions(ctx context.Context, userID string, at time.Time) error {
	return endSessions(ctx, w.tx, userID, at)
}

func endSessions(ctx context.Context, tx pgx.Tx, userID string, at time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE app.users SET session_epoch = session_epoch + 1 WHERE id = $1::uuid`, userID); err != nil {
		return fmt.Errorf("move session epoch: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE app.refresh_tokens
		   SET revoked_at = $2
		 WHERE user_id = $1::uuid AND revoked_at IS NULL`, userID, at); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	return nil
}

func (w *linkWrites) Enqueue(ctx context.Context, m domain.LinkMail) error {
	if w.outbox == nil {
		return domain.ErrSetPasswordUnavailable
	}
	mail := email.Mail{ToUserID: m.UserID, To: m.To, Secret: m.Token}
	switch m.Kind {
	case domain.MailAccountInvite:
		mail.Params = email.AccountInviteParams{FullName: m.FullName}
	case domain.MailPasswordReset:
		mail.Params = email.PasswordResetLinkParams{FullName: m.FullName}
	case domain.MailSignInInvite:
		mail.Params = email.SignInInviteParams{FullName: m.FullName}
	default:
		return fmt.Errorf("set-password mail of kind %d", m.Kind)
	}
	if _, _, err := w.outbox.Enqueue(ctx, w.tx, mail); err != nil {
		if errors.Is(err, email.ErrNoSecretsKey) {
			return domain.ErrSetPasswordUnavailable
		}
		return fmt.Errorf("enqueue set-password mail: %w", err)
	}
	if !w.woken {
		w.woken = true
		w.after = append(w.after, w.outbox.Notify)
	}
	return nil
}

func (w *linkWrites) Audit(ctx context.Context, entry audit.Entry) error {
	return audit.Write(ctx, w.tx, entry)
}

// Find reads the link a token names with its account, in any state.
func (s *SetPasswordLinks) Find(ctx context.Context, tokenHash []byte) (domain.SetPasswordLink, error) {
	var (
		l       domain.SetPasswordLink
		purpose string
	)
	err := s.QueryRow(ctx, `
		SELECT t.user_id::text, u.email, u.full_name, t.purpose, t.expires_at, t.used_at, t.revoked_at,
		       u.disabled_at IS NOT NULL
		  FROM app.set_password_tokens t
		  JOIN app.users u ON u.id = t.user_id
		 WHERE t.token_hash = $1`, tokenHash).Scan(
		&l.UserID, &l.Email, &l.FullName, &purpose, &l.ExpiresAt, &l.UsedAt, &l.RevokedAt, &l.AccountDisabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SetPasswordLink{}, domain.ErrSetPasswordLinkInvalid
	}
	if err != nil {
		return domain.SetPasswordLink{}, fmt.Errorf("find set-password link: %w", err)
	}
	l.Purpose = domain.SetPasswordPurpose(purpose)
	return l, nil
}

// Claim uses the link and sets the password of its account. One statement
// claims the token, so of two claims of one link the second reads no row and
// changes nothing; a disabled account rolls the claim back, leaving the link
// as it was.
func (s *SetPasswordLinks) Claim(ctx context.Context, in domain.ClaimLink) (domain.ClaimedLink, error) {
	var claimed domain.ClaimedLink
	err := s.Context.InTx(ctx, "identity: claim set-password link", func(tx pgx.Tx) error {
		var purpose string
		err := tx.QueryRow(ctx, `
			UPDATE app.set_password_tokens
			   SET used_at = $2
			 WHERE token_hash = $1 AND used_at IS NULL AND revoked_at IS NULL AND expires_at > $2
			RETURNING user_id::text, purpose`, in.TokenHash, in.Now).Scan(&claimed.UserID, &purpose)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrSetPasswordLinkInvalid
		}
		if err != nil {
			return fmt.Errorf("claim set-password link: %w", err)
		}
		claimed.Purpose = domain.SetPasswordPurpose(purpose)

		tag, err := tx.Exec(ctx, `
			UPDATE app.users
			   SET password_hash = $2, must_change_password = false, password_changed_at = $3,
			       session_epoch = session_epoch + 1
			 WHERE id = $1::uuid AND disabled_at IS NULL`, claimed.UserID, in.NewHash, in.Now)
		if err != nil {
			return fmt.Errorf("set password: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrSetPasswordLinkInvalid
		}
		if _, err := tx.Exec(ctx, revokeUnusedLinks, claimed.UserID, in.Now); err != nil {
			return fmt.Errorf("revoke other set-password links: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE app.refresh_tokens
			   SET revoked_at = $2
			 WHERE user_id = $1::uuid AND revoked_at IS NULL`, claimed.UserID, in.Now); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
		diff, err := json.Marshal(struct {
			Purpose string `json:"purpose"`
		}{purpose})
		if err != nil {
			return fmt.Errorf("encode audit diff: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			ActorUserID: &claimed.UserID,
			Action:      "user.password_set",
			Entity:      entityUser,
			EntityID:    &claimed.UserID,
			OccurredAt:  in.Now,
			IP:          in.IP,
			UserAgent:   in.UserAgent,
			Diff:        diff,
		})
	})
	if err != nil {
		return domain.ClaimedLink{}, err
	}
	return claimed, nil
}

// Prune deletes at most limit links that were used, revoked or expired before
// the given moment, whichever of the three came first. The table holds the
// links of the last 30 days and nothing indexes the three columns, so the scan
// reads that many rows at most; the cascade from app.users needs only
// set_password_tokens_user_idx. It skips a link another transaction has
// locked, so it never waits on a claim, an issue or a disable and cannot form
// a cycle with them; a batch that skipped one comes back short and the next
// run takes it.
func (s *SetPasswordLinks) Prune(ctx context.Context, before time.Time, limit int) (int, error) {
	tag, err := s.Exec(ctx, `
		DELETE FROM app.set_password_tokens t
		 USING (SELECT id FROM app.set_password_tokens
		         WHERE least(used_at, revoked_at, expires_at) < $1
		         ORDER BY least(used_at, revoked_at, expires_at), id
		         LIMIT $2
		         FOR UPDATE SKIP LOCKED) old
		 WHERE t.id = old.id`, before, limit)
	if err != nil {
		return 0, fmt.Errorf("prune set-password links: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
