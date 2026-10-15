// Package repositories stores data exports in PostgreSQL and reads the tables
// an export holds. Its Postgres is both the API's repository and the worker's
// queue, and the Source the worker streams the files from.
package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/exports/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/email"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
)

// Postgres keeps exports in app.data_exports. Without WithOutbox a completed
// export tells nobody.
type Postgres struct {
	db.Repository
	outbox *email.Outbox
}

// NewPostgres keeps exports through dbx.
func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

// WithOutbox makes Complete enqueue the ready notice into outbox, in its own
// transaction, and wake outbox's dispatcher after the commit.
func (p *Postgres) WithOutbox(outbox *email.Outbox) *Postgres {
	p.outbox = outbox
	return p
}

var (
	_ domain.Repository = (*Postgres)(nil)
	_ domain.Queue      = (*Postgres)(nil)
	_ domain.Source     = (*Postgres)(nil)
)

const (
	activeIndex = "data_exports_active_idx"

	columns = `e.id::text, e.requested_by::text, e.include_answers,
	           CASE WHEN e.status = 'ready' AND e.expires_at <= now() THEN 'expired' ELSE e.status END,
	           e.attempts, e.bytes, e.created_at, e.completed_at, e.expires_at, e.downloaded_at, e.downloads`
)

func scan(row pgx.Row) (domain.Export, error) {
	var (
		out    domain.Export
		status string
	)
	err := row.Scan(&out.ID, &out.RequestedBy, &out.IncludeAnswers, &status, &out.Attempts, &out.Bytes,
		&out.CreatedAt, &out.CompletedAt, &out.ExpiresAt, &out.DownloadedAt, &out.Downloads)
	if err != nil {
		return domain.Export{}, err
	}
	out.Status = domain.Status(status)
	return out, nil
}

// Request queues an export for in.ActorID and audits it. The actor's own
// exports are checked inside the transaction that inserts, and two requests at
// once are still told apart by the active index.
func (p *Postgres) Request(ctx context.Context, in domain.Request) (domain.Export, error) {
	var out domain.Export
	err := p.InTx(ctx, "exports: request", func(tx pgx.Tx) error {
		if err := tooSoon(ctx, tx, in.ActorID); err != nil {
			return err
		}
		var err error
		out, err = scan(tx.QueryRow(ctx, `
			INSERT INTO app.data_exports AS e (requested_by, include_answers)
			VALUES ($1, $2)
			RETURNING `+columns, in.ActorID, in.IncludeAnswers))
		if db.IsUniqueViolation(err, activeIndex) {
			return domain.ErrInProgress
		}
		if err != nil {
			return fmt.Errorf("exports: queue the export: %w", err)
		}
		return auditRequested(ctx, tx, in, out.ID)
	})
	if err != nil {
		return domain.Export{}, err
	}
	return out, nil
}

func tooSoon(ctx context.Context, q db.Querier, actor string) error {
	var wait int64
	err := q.QueryRow(ctx, `
		SELECT ceil(extract(epoch FROM e.created_at + $2 * interval '1 second' - now()))::bigint
		  FROM app.data_exports e
		 WHERE e.requested_by = $1::uuid AND e.status <> 'failed'
		   AND e.created_at > now() - $2 * interval '1 second'
		 ORDER BY e.created_at DESC
		 LIMIT 1`, actor, int64(domain.RequestInterval/time.Second)).Scan(&wait)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("exports: look for a recent export: %w", err)
	}
	return &domain.TooSoonError{RetryAfter: time.Duration(max(wait, 1)) * time.Second}
}

// Get reads requester's export.
func (p *Postgres) Get(ctx context.Context, id, requester string) (domain.Export, error) {
	out, err := scan(p.QueryRow(ctx, `
		SELECT `+columns+` FROM app.data_exports e WHERE e.id = $1::uuid AND e.requested_by = $2::uuid`, id, requester))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Export{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Export{}, fmt.Errorf("exports: read the export: %w", err)
	}
	return out, nil
}

// Fetch counts a download of the export, signs its file and audits the fetch.
func (p *Postgres) Fetch(ctx context.Context, in domain.Fetch, sign domain.Signer) (string, error) {
	var url string
	err := p.InTx(ctx, "exports: fetch", func(tx pgx.Tx) error {
		var (
			key     string
			created time.Time
		)
		err := tx.QueryRow(ctx, `
			UPDATE app.data_exports
			   SET downloaded_at = coalesce(downloaded_at, now()), downloads = downloads + 1
			 WHERE id = $1::uuid AND requested_by = $2::uuid
			   AND status = 'ready' AND expires_at > now() AND downloads < $3
			RETURNING storage_key, created_at`, in.ID, in.ActorID, domain.MaxDownloads).Scan(&key, &created)
		if errors.Is(err, pgx.ErrNoRows) {
			return p.whyNot(ctx, tx, in)
		}
		if err != nil {
			return fmt.Errorf("exports: mark the export fetched: %w", err)
		}
		if url, err = sign(key, created); err != nil {
			return fmt.Errorf("exports: sign the export: %w", err)
		}
		return auditFetched(ctx, tx, in)
	})
	if err != nil {
		return "", err
	}
	return url, nil
}

func (p *Postgres) whyNot(ctx context.Context, q db.Querier, in domain.Fetch) error {
	exists, err := db.Exists(ctx, q, `SELECT 1 FROM app.data_exports WHERE id = $1::uuid AND requested_by = $2::uuid`, in.ID, in.ActorID)
	if err != nil {
		return fmt.Errorf("exports: look for the export: %w", err)
	}
	if exists {
		return domain.ErrUnavailable
	}
	return domain.ErrNotFound
}

func auditRequested(ctx context.Context, tx pgx.Tx, in domain.Request, id string) error {
	diff, err := json.Marshal(struct {
		IncludeAnswers bool `json:"includeAnswers"`
	}{in.IncludeAnswers})
	if err != nil {
		return fmt.Errorf("exports: encode audit diff: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{
		ActorUserID: opt.String(in.ActorID), Action: "data_export.requested", Entity: "data_export", EntityID: &id,
		OccurredAt: time.Now(), IP: opt.String(in.IP), UserAgent: opt.String(in.UserAgent), Diff: diff,
	})
}

func auditFetched(ctx context.Context, tx pgx.Tx, in domain.Fetch) error {
	id := in.ID
	return audit.Write(ctx, tx, audit.Entry{
		ActorUserID: opt.String(in.ActorID), Action: "data_export.downloaded", Entity: "data_export", EntityID: &id,
		OccurredAt: time.Now(), IP: opt.String(in.IP), UserAgent: opt.String(in.UserAgent),
	})
}
