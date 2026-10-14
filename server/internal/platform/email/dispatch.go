package email

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/secrets"
)

// BatchSize is the most rows one Dispatch claims.
const BatchSize = 10

// MaxAttempts is how many sends a message gets before it is marked failed.
const MaxAttempts = 10

const commitBudget = 10 * time.Second

// Dispatcher sends what the outbox holds. The API and the worker each run
// one; a transaction advisory lock (73819, 43) lets one of them dispatch at a
// time, so the daily budget and the provider's request rate hold across both.
//
// Sealer opens the rows' secrets; a nil one fails every row with a secret.
// Org answers the organization's short name for the header. DailyLimit is the
// most messages sent in one local day of Location, and Interval the pause
// between two sends. Now is the clock, time.Now when nil.
type Dispatcher struct {
	DB         db.Context
	Sealer     *secrets.Sealer
	Sender     Sender
	Renderer   *Renderer
	Org        func(context.Context) (string, error)
	AppURL     string
	ReplyTo    string
	DailyLimit int
	Interval   time.Duration
	Location   *time.Location
	Now        func() time.Time
	Logger     *slog.Logger
}

// Batch is what one Dispatch did. Busy means another dispatcher held the
// lock and nothing was claimed; Claimed counts the rows it locked, and the
// rest count outcomes, Deferred being rows moved to the next local day
// because the day's budget was spent.
type Batch struct {
	Busy     bool
	Claimed  int
	Sent     int
	Retried  int
	Failed   int
	Deferred int
}

type pending struct {
	id       string
	to       string
	template string
	locale   string
	params   []byte
	secret   []byte
	attempts int
}

type outcome struct {
	provider string
	code     string
	kind     error
}

// Dispatch claims up to BatchSize due rows and sends them, Interval apart. A
// sent row and a failed one lose their secret in the statement that records
// them. A transient failure is retried after 1, 5 and 15 minutes and then
// hourly, until the MaxAttempts-th attempt fails the row. Once the day's
// budget is spent, every due row moves to the next local midnight. When ctx
// ends, no further row is sent and what was sent is still recorded.
func (d *Dispatcher) Dispatch(ctx context.Context) (Batch, error) {
	org, err := d.Org(ctx)
	if err != nil {
		return Batch{}, err
	}
	tx, err := d.DB.Begin(ctx)
	if err != nil {
		return Batch{}, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(73819, 43)`).Scan(&locked); err != nil {
		return Batch{}, err
	}
	if !locked {
		return Batch{Busy: true}, nil
	}
	now := d.now()
	dayStart, nextDay := localDay(now, d.Location)
	var sentToday int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM app.email_outbox WHERE sent_at >= $1 AND sent_at < $2`, dayStart, nextDay).Scan(&sentToday); err != nil {
		return Batch{}, err
	}
	var batch Batch
	if remaining := d.DailyLimit - sentToday; remaining <= 0 {
		tag, err := tx.Exec(ctx, `
			UPDATE app.email_outbox SET deliver_after = $2
			 WHERE sent_at IS NULL AND failed_at IS NULL AND deliver_after <= $1`, now, nextDay)
		if err != nil {
			return Batch{}, err
		}
		batch.Deferred = int(tag.RowsAffected())
	} else if err := d.sendDue(ctx, tx, now, min(BatchSize, remaining), org, &batch); err != nil {
		return Batch{}, err
	}
	commit, cancel := context.WithTimeout(context.WithoutCancel(ctx), commitBudget)
	defer cancel()
	if err := tx.Commit(commit); err != nil {
		return Batch{}, err
	}
	if batch.Deferred > 0 {
		d.Logger.Info("email budget spent for the day", "deferred", batch.Deferred, "limit", d.DailyLimit, "until", nextDay)
	}
	return batch, nil
}

func (d *Dispatcher) sendDue(ctx context.Context, tx pgx.Tx, now time.Time, limit int, org string, batch *Batch) error {
	rows, err := db.QueryMany(ctx, tx, `
		SELECT id::text, to_address, template, locale, params, secret_ciphertext, attempts
		  FROM app.email_outbox
		 WHERE sent_at IS NULL AND failed_at IS NULL AND deliver_after <= $1
		 ORDER BY deliver_after, id
		 LIMIT $2
		   FOR UPDATE SKIP LOCKED`, []any{now, limit}, func(r pgx.Rows) (pending, error) {
		var p pending
		err := r.Scan(&p.id, &p.to, &p.template, &p.locale, &p.params, &p.secret, &p.attempts)
		return p, err
	})
	if err != nil {
		return err
	}
	batch.Claimed = len(rows)
	record := context.WithoutCancel(ctx)
	for i, row := range rows {
		if i > 0 && !pause(ctx, d.Interval) {
			break
		}
		if ctx.Err() != nil {
			break
		}
		result := d.deliver(ctx, row, org)
		if err := d.record(record, tx, row, result, batch); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dispatcher) deliver(ctx context.Context, row pending, org string) outcome {
	params, err := DecodeParams(Template(row.template), row.params)
	if err != nil {
		return outcome{kind: ErrPermanent, code: "params_unreadable"}
	}
	var secret string
	if row.secret != nil {
		if d.Sealer == nil {
			return outcome{kind: ErrPermanent, code: "secret_unreadable"}
		}
		opened, err := d.Sealer.Open(row.secret, []byte(row.id))
		if err != nil {
			return outcome{kind: ErrPermanent, code: "secret_unreadable"}
		}
		secret = string(opened)
	}
	rendered, err := d.Renderer.Render(Letter{Locale: row.locale, Org: org, AppURL: d.AppURL, Params: params, Secret: secret})
	if err != nil {
		return outcome{kind: ErrPermanent, code: "render_failed"}
	}
	provider, err := d.Sender.Send(ctx, Message{
		To: row.to, Subject: rendered.Subject, HTML: rendered.HTML, Text: rendered.Text, ReplyTo: d.ReplyTo,
		IdempotencyKey: row.id, Tags: map[string]string{"template": row.template},
	})
	if err != nil {
		var failure *SendError
		if errors.As(err, &failure) {
			return outcome{kind: failure.Kind, code: failure.Code}
		}
		return outcome{kind: ErrTransient, code: "send_failed"}
	}
	return outcome{provider: provider}
}

func (d *Dispatcher) record(ctx context.Context, tx pgx.Tx, row pending, result outcome, batch *Batch) error {
	at := d.now()
	attempts := row.attempts + 1
	var err error
	verdict := "sent"
	switch {
	case result.kind == nil:
		batch.Sent++
		_, err = tx.Exec(ctx, `
			UPDATE app.email_outbox
			   SET sent_at = $2, provider_id = $3, attempts = $4, last_error_code = NULL, secret_ciphertext = NULL
			 WHERE id = $1`, row.id, at, truncate(result.provider, 200), attempts)
	case errors.Is(result.kind, ErrTransient) && attempts < MaxAttempts:
		batch.Retried++
		verdict = "retry"
		_, err = tx.Exec(ctx, `
			UPDATE app.email_outbox SET attempts = $2, last_error_code = $3, deliver_after = $4 WHERE id = $1`,
			row.id, attempts, truncate(result.code, 64), at.Add(Backoff(attempts)))
	default:
		batch.Failed++
		verdict = "failed"
		_, err = tx.Exec(ctx, `
			UPDATE app.email_outbox
			   SET failed_at = $2, attempts = $3, last_error_code = $4, secret_ciphertext = NULL
			 WHERE id = $1`, row.id, at, attempts, truncate(result.code, 64))
	}
	if err != nil {
		return err
	}
	d.Logger.Info("email dispatched", "outboxId", row.id, "template", row.template, "outcome", verdict,
		"code", result.code, "attempt", attempts, "toDomain", Domain(row.to))
	return nil
}

// Backoff is the wait before a message's next attempt after its attempts-th
// failed transiently: 1, 5 and 15 minutes, then an hour.
func Backoff(attempts int) time.Duration {
	switch attempts {
	case 1:
		return time.Minute
	case 2:
		return 5 * time.Minute
	case 3:
		return 15 * time.Minute
	default:
		return time.Hour
	}
}

// NextDue answers when the earliest pending row falls due, and false when
// nothing is pending.
func (d *Dispatcher) NextDue(ctx context.Context) (time.Time, bool, error) {
	var due *time.Time
	if err := d.DB.QueryRow(ctx, `
		SELECT min(deliver_after) FROM app.email_outbox WHERE sent_at IS NULL AND failed_at IS NULL`).Scan(&due); err != nil {
		return time.Time{}, false, err
	}
	if due == nil {
		return time.Time{}, false, nil
	}
	return *due, true, nil
}

func (d *Dispatcher) now() time.Time {
	if d.Now == nil {
		return time.Now()
	}
	return d.Now()
}

func localDay(now time.Time, location *time.Location) (time.Time, time.Time) {
	if location == nil {
		location = time.UTC
	}
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return start, start.AddDate(0, 0, 1)
}

func pause(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
