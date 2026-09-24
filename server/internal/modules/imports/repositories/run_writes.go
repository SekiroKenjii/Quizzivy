package repositories

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/imports/domain"
	"regexp"
	"slices"
	"time"
)

const runFailed = "failed"

func (s *Postgres) Heartbeat(ctx context.Context, c domain.Claim, lease time.Duration) error {
	if lease < time.Second || lease > 5*time.Minute {
		return domain.ErrConflict
	}
	return s.InTx(ctx, "heartbeat import run", func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73819,11)`); err != nil {
			return err
		}
		if _, err := lockedClaim(ctx, tx, c); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE app.word_import_runs SET lease_until=clock_timestamp()+$2*interval '1 millisecond' WHERE id=$1`, c.RunID, lease.Milliseconds())
		return err
	})
}
func (s *Postgres) Progress(ctx context.Context, c domain.Claim, stage string) error {
	stages := []string{"source_validation", "normalization", "extraction", "recognition", "validation"}
	position := slices.Index(stages, stage)
	if position < 0 {
		return domain.ErrConflict
	}
	return s.InTx(ctx, "advance import stage", func(tx pgx.Tx) error {
		run, err := lockedClaim(ctx, tx, c)
		if err != nil {
			return err
		}
		if position < slices.Index(stages, run.Stage) {
			return domain.ErrConflict
		}
		if run.Stage == stage {
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE app.word_import_runs SET stage=$2 WHERE id=$1`, c.RunID, stage); err != nil {
			return err
		}
		run.Stage = stage
		return recordRunEvent(ctx, tx, run, "stage", "", nil)
	})
}

func (s *Postgres) Complete(ctx context.Context, c domain.Claim, result json.RawMessage) error {
	if err := domain.ValidateRunResult(result); err != nil {
		return err
	}
	return s.InTx(ctx, "complete import run", func(tx pgx.Tx) error {
		run, err := lockedClaim(ctx, tx, c)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.word_import_runs SET status='succeeded',stage='ready',worker_id=NULL,lease_until=NULL,result=$2,completed_at=clock_timestamp() WHERE id=$1`, c.RunID, result); err != nil {
			return err
		}
		run.Stage = "ready"
		if err := recordRunEvent(ctx, tx, run, "completed", "", nil); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE app.word_imports SET status='needs_review',revision=revision+1 WHERE id=$1`, c.ImportID)
		return err
	})
}

var runErrorCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,99}$`)

func (s *Postgres) Fail(ctx context.Context, in domain.RunFailure) error {
	if !runErrorCode.MatchString(in.Code) || in.RetryAfter < 0 || in.RetryAfter > time.Hour {
		return domain.ErrConflict
	}
	return s.InTx(ctx, "fail import run", func(tx pgx.Tx) error {
		run, err := lockedClaim(ctx, tx, in.Claim)
		if err != nil {
			return err
		}
		retry := in.Retryable && run.AttemptCount < run.MaxAttempts
		runState, parentState := runFailed, runFailed
		if retry {
			runState, parentState = "queued", "queued"
		}
		if _, err := tx.Exec(ctx, `UPDATE app.word_import_runs SET status=$2,worker_id=NULL,lease_until=NULL,error_code=$3,
  available_at=clock_timestamp()+$4*interval '1 millisecond',completed_at=CASE WHEN $2='failed' THEN clock_timestamp() ELSE NULL END WHERE id=$1`, run.ID, runState, in.Code, in.RetryAfter.Milliseconds()); err != nil {
			return err
		}
		kind := runFailed
		if retry {
			kind = "retry_scheduled"
		}
		if err := recordRunEvent(ctx, tx, run, kind, in.Code, nil); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE app.word_imports SET status=$2,revision=revision+1 WHERE id=$1`, run.ImportID, parentState)
		return err
	})
}
