package repositories

import (
	"context"
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

func (s *Postgres) Complete(ctx context.Context, c domain.Claim, outcome domain.Outcome) error {
	if err := domain.ValidateRunResult(outcome.Result); err != nil {
		return err
	}
	return s.InTx(ctx, "complete import run", func(tx pgx.Tx) error {
		run, err := lockedClaim(ctx, tx, c)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.word_import_runs SET status='succeeded',stage='ready',worker_id=NULL,lease_until=NULL,result=$2,completed_at=clock_timestamp() WHERE id=$1`, c.RunID, outcome.Result); err != nil {
			return err
		}
		if err := storeMachineDraft(ctx, tx, c, outcome.Draft); err != nil {
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

type failureOutcome struct {
	status string
	event  string
	delay  time.Duration
	refund int
}

func nextAfterFailure(in domain.RunFailure, run domain.Run) failureOutcome {
	switch {
	case in.Released:
		return failureOutcome{status: "queued", event: "retry_scheduled", refund: 1}
	case in.Retryable && run.AttemptCount < run.MaxAttempts:
		return failureOutcome{status: "queued", event: "retry_scheduled", delay: in.RetryAfter}
	default:
		return failureOutcome{status: runFailed, event: runFailed, delay: in.RetryAfter}
	}
}

func (s *Postgres) Fail(ctx context.Context, in domain.RunFailure) error {
	if !runErrorCode.MatchString(in.Code) || in.RetryAfter < 0 || in.RetryAfter > time.Hour {
		return domain.ErrConflict
	}
	return s.InTx(ctx, "fail import run", func(tx pgx.Tx) error {
		run, err := lockedClaim(ctx, tx, in.Claim)
		if err != nil {
			return err
		}
		next := nextAfterFailure(in, run)
		if _, err := tx.Exec(ctx, `UPDATE app.word_import_runs SET status=$2,worker_id=NULL,lease_until=NULL,error_code=$3,attempt_count=attempt_count-$5,
  available_at=clock_timestamp()+$4*interval '1 millisecond',completed_at=CASE WHEN $2='failed' THEN clock_timestamp() ELSE NULL END WHERE id=$1`, run.ID, next.status, in.Code, next.delay.Milliseconds(), next.refund); err != nil {
			return err
		}
		if err := recordRunEvent(ctx, tx, run, next.event, in.Code, nil); err != nil {
			return err
		}
		if next.status == runFailed {
			_, err = tx.Exec(ctx, failImport, run.ImportID)
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE app.word_imports SET status=$2,revision=revision+1 WHERE id=$1`, run.ImportID, next.status)
		return err
	})
}
