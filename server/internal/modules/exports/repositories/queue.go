package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/exports/domain"
	"quizzivy/internal/platform/email"
)

const leaseSeconds = int64(domain.Lease / time.Second)

// Running says which of ids are running now.
func (p *Postgres) Running(ctx context.Context, ids []string) ([]string, error) {
	rows, err := p.Query(ctx, `SELECT id::text FROM app.data_exports WHERE status = 'running' AND id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, fmt.Errorf("exports: look for the running: %w", err)
	}
	defer rows.Close()
	var running []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("exports: read a running export: %w", err)
		}
		running = append(running, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("exports: read the running: %w", err)
	}
	return running, nil
}

// Claim takes the oldest waiting export for the worker.
func (p *Postgres) Claim(ctx context.Context) (domain.Export, error) {
	var (
		out   domain.Export
		found bool
	)
	err := p.InTx(ctx, "exports: claim", func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE app.data_exports
			   SET status = 'failed', error_code = $1, completed_at = now()
			 WHERE status = 'running' AND attempts >= $2
			   AND started_at < now() - $3 * interval '1 second'`,
			domain.CodeInterrupted, domain.MaxAttempts, leaseSeconds); err != nil {
			return fmt.Errorf("exports: fail the interrupted: %w", err)
		}
		claimed, err := scan(tx.QueryRow(ctx, `
			WITH due AS (
			  SELECT id FROM app.data_exports
			   WHERE status = 'queued'
			      OR (status = 'running' AND started_at < now() - $1 * interval '1 second')
			   ORDER BY created_at
			   LIMIT 1
			     FOR UPDATE SKIP LOCKED)
			UPDATE app.data_exports e
			   SET status = 'running', started_at = now(), attempts = e.attempts + 1
			  FROM due
			 WHERE e.id = due.id
			RETURNING `+columns, leaseSeconds))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("exports: claim: %w", err)
		}
		out, found = claimed, true
		return nil
	})
	if err != nil {
		return domain.Export{}, err
	}
	if !found {
		return domain.Export{}, domain.ErrNoWork
	}
	return out, nil
}

// NextDue says when the waiting export falls due.
func (p *Postgres) NextDue(ctx context.Context) (time.Time, bool, error) {
	var due time.Time
	err := p.QueryRow(ctx, `
		SELECT CASE WHEN status = 'queued' THEN now() ELSE started_at + $1 * interval '1 second' END
		  FROM app.data_exports
		 WHERE status IN ('queued', 'running')
		 ORDER BY created_at
		 LIMIT 1`, leaseSeconds).Scan(&due)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("exports: next due: %w", err)
	}
	return due, true, nil
}

// Complete marks the export ready and enqueues the requester's notice.
func (p *Postgres) Complete(ctx context.Context, done domain.Completion) error {
	err := p.InTx(ctx, "exports: complete", func(tx pgx.Tx) error {
		var requester string
		var expires time.Time
		err := tx.QueryRow(ctx, `
			UPDATE app.data_exports
			   SET status = 'ready', storage_key = $3, bytes = $4, error_code = NULL,
			       completed_at = now(), expires_at = now() + $5 * interval '1 second'
			 WHERE id = $1::uuid AND status = 'running' AND attempts = $2
			RETURNING requested_by::text, expires_at`,
			done.ID, done.Attempt, done.StorageKey, done.Bytes, int64(domain.KeepFor/time.Second)).Scan(&requester, &expires)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrLeaseLost
		}
		if err != nil {
			return fmt.Errorf("exports: mark the export ready: %w", err)
		}
		return p.tell(ctx, tx, done.ID, requester, expires)
	})
	if err != nil {
		return err
	}
	if p.outbox != nil {
		p.outbox.Notify()
	}
	return nil
}

func (p *Postgres) tell(ctx context.Context, tx pgx.Tx, id, requester string, expires time.Time) error {
	if p.outbox == nil {
		return nil
	}
	var address, name string
	if err := tx.QueryRow(ctx, `SELECT email, full_name FROM app.users WHERE id = $1::uuid`, requester).Scan(&address, &name); err != nil {
		return fmt.Errorf("exports: read the requester: %w", err)
	}
	_, _, err := p.outbox.Enqueue(ctx, tx, email.Mail{
		ToUserID:  requester,
		To:        address,
		Params:    email.DataExportReadyParams{FullName: name, ExportID: id},
		DedupeKey: "data_export_ready:" + id,
		ExpiresAt: expires,
	})
	if err != nil {
		return fmt.Errorf("exports: enqueue the ready notice: %w", err)
	}
	return nil
}

// Fail marks the claimed export failed with code.
func (p *Postgres) Fail(ctx context.Context, id string, attempt int, code string) error {
	tag, err := p.Exec(ctx, `
		UPDATE app.data_exports
		   SET status = 'failed', error_code = $3, completed_at = now()
		 WHERE id = $1::uuid AND status = 'running' AND attempts = $2`, id, attempt, code)
	if err != nil {
		return fmt.Errorf("exports: fail the export: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLeaseLost
	}
	return nil
}

// Release gives the claimed export back without spending the attempt.
func (p *Postgres) Release(ctx context.Context, id string, attempt int) error {
	tag, err := p.Exec(ctx, `
		UPDATE app.data_exports
		   SET status = 'queued', started_at = NULL, attempts = attempts - 1
		 WHERE id = $1::uuid AND status = 'running' AND attempts = $2`, id, attempt)
	if err != nil {
		return fmt.Errorf("exports: release the export: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLeaseLost
	}
	return nil
}
