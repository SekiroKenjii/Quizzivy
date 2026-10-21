package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/secrets"
)

// The ways Enqueue refuses a message before writing it.
var (
	ErrInvalidAddress = errors.New("email: the recipient is not one email address")
	ErrInvalidLocale  = errors.New("email: the locale is neither vi nor en")
	ErrSecretRequired = errors.New("email: the template needs a secret")
	ErrSecretUnused   = errors.New("email: the template carries no secret")
	ErrNoSecretsKey   = errors.New("email: SECRETS_KEY is not configured, so a secret cannot be sealed")
)

// Mail is a message to enqueue. ToUserID, Locale, Secret, DedupeKey,
// DeliverAfter and ExpiresAt are optional: an empty Locale is the recipient's
// own (users.locale) or else vi, a second Mail with a DedupeKey already in the
// outbox is not written, a zero DeliverAfter is now, and ExpiresAt is when the
// message's link stops working, after which it is failed and never sent.
type Mail struct {
	ToUserID     string
	To           string
	Locale       string
	Params       Params
	Secret       string
	DedupeKey    string
	DeliverAfter time.Time
	ExpiresAt    time.Time
}

// Outbox writes messages into app.email_outbox and wakes this process's
// dispatcher. A nil sealer is an outbox without SECRETS_KEY, which refuses
// any Mail with a secret.
type Outbox struct {
	sealer *secrets.Sealer
	wake   chan struct{}
}

// NewOutbox is the outbox over sealer.
func NewOutbox(sealer *secrets.Sealer) *Outbox {
	return &Outbox{sealer: sealer, wake: make(chan struct{}, 1)}
}

// Enqueue writes m on q, which is the caller's transaction, so a rolled-back
// caller sends nothing. A secret is sealed with the row's id as associated
// data. It answers the row's id and whether this call wrote it, which is false
// when m's DedupeKey was already in the outbox. The caller calls Notify after
// its transaction commits.
func (o *Outbox) Enqueue(ctx context.Context, q db.Querier, m Mail) (string, bool, error) {
	t, address, err := o.validate(m)
	if err != nil {
		return "", false, err
	}
	params, err := json.Marshal(m.Params)
	if err != nil {
		return "", false, fmt.Errorf("email: params: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return "", false, err
	}
	var sealed []byte
	if m.Secret != "" {
		if sealed, err = o.sealer.Seal([]byte(m.Secret), []byte(id.String())); err != nil {
			return "", false, err
		}
	}
	var written string
	err = q.QueryRow(ctx, `
		INSERT INTO app.email_outbox
		       (id, to_user_id, to_address, template, locale, params, secret_ciphertext, dedupe_key, deliver_after, expires_at)
		VALUES ($1, $2::uuid, $3, $4,
		        COALESCE($5::text, (SELECT u.locale FROM app.users u WHERE u.id = $2::uuid), 'vi'),
		        $6, $7, $8, COALESCE($9::timestamptz, now()), $10)
		ON CONFLICT ON CONSTRAINT email_outbox_dedupe_key DO NOTHING
		RETURNING id::text`,
		id, nullable(m.ToUserID), address.Address, string(t), nullable(m.Locale), params, sealed,
		nullable(m.DedupeKey), nullableTime(m.DeliverAfter), nullableTime(m.ExpiresAt)).Scan(&written)
	if errors.Is(err, pgx.ErrNoRows) {
		err = q.QueryRow(ctx, `SELECT id::text FROM app.email_outbox WHERE dedupe_key = $1`, m.DedupeKey).Scan(&written)
		return written, false, err
	}
	if err != nil {
		return "", false, err
	}
	return written, true, nil
}

func (o *Outbox) validate(m Mail) (Template, *mail.Address, error) {
	if m.Params == nil {
		return "", nil, ErrUnknownTemplate
	}
	t := m.Params.Template()
	if _, ok := definitions[t]; !ok {
		return "", nil, ErrUnknownTemplate
	}
	address, err := mail.ParseAddress(strings.TrimSpace(m.To))
	if err != nil || address.Name != "" || len(address.Address) < 3 || len(address.Address) > 254 {
		return "", nil, ErrInvalidAddress
	}
	if m.Locale != "" && m.Locale != "vi" && m.Locale != "en" {
		return "", nil, ErrInvalidLocale
	}
	switch {
	case NeedsSecret(t) && m.Secret == "":
		return "", nil, ErrSecretRequired
	case !NeedsSecret(t) && m.Secret != "":
		return "", nil, ErrSecretUnused
	case m.Secret != "" && o.sealer == nil:
		return "", nil, ErrNoSecretsKey
	}
	return t, address, nil
}

// Retention is how long a row stays in the outbox once it is sent or failed.
// A pruned row frees its dedupe key, so a producer whose key can come round
// again later than this would send the message again.
const Retention = 30 * 24 * time.Hour

// Prune deletes at most limit rows that were sent or failed more than
// Retention before now, and reports how many went. A row still waiting is
// never touched. The table holds a day's budget of rows a day, so the scan for
// failed rows, which has no index, reads a few thousand at most.
func (o *Outbox) Prune(ctx context.Context, q db.Querier, now time.Time, limit int) (int, error) {
	if limit <= 0 {
		return 0, errors.New("email: prune needs a positive limit")
	}
	tag, err := q.Exec(ctx, `
		DELETE FROM app.email_outbox o
		 USING (SELECT id FROM app.email_outbox
		         WHERE sent_at < $1 OR failed_at < $1
		         ORDER BY coalesce(sent_at, failed_at)
		         LIMIT $2) old
		 WHERE o.id = old.id`, now.Add(-Retention), limit)
	if err != nil {
		return 0, fmt.Errorf("email: prune the outbox: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// CancelPending ends, on q, every message still waiting for the user as a
// failed one with the code "cancelled" and its sealed secret erased, and
// reports how many it ended. A message a dispatcher holds locked while it sends
// is not waited for and not recalled: it goes out, as it would have had the
// caller been a moment later.
func CancelPending(ctx context.Context, q db.Querier, userID string, at time.Time) (int, error) {
	tag, err := q.Exec(ctx, `
		UPDATE app.email_outbox o
		   SET failed_at = $2, last_error_code = 'cancelled', secret_ciphertext = NULL
		 WHERE o.id IN (SELECT w.id FROM app.email_outbox w
		                 WHERE w.to_user_id = $1::uuid AND w.sent_at IS NULL AND w.failed_at IS NULL
		                   FOR UPDATE SKIP LOCKED)`, userID, at)
	if err != nil {
		return 0, fmt.Errorf("email: cancel the messages waiting for a user: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// Notify wakes this process's dispatcher. Signals coalesce, and it never
// blocks.
func (o *Outbox) Notify() {
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

// Wake is the channel Notify signals on, for the dispatcher's loop.
func (o *Outbox) Wake() <-chan struct{} { return o.wake }

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
