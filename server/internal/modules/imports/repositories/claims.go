package repositories

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/db"
	"time"
)

func (s *Postgres) Claim(ctx context.Context, p domain.ClaimPolicy) (domain.Run, error) {
	if p.Lease < time.Second || p.Lease > 5*time.Minute || p.GlobalLimit < 1 || p.ActorLimit < 1 {
		return domain.Run{}, domain.ErrConflict
	}
	var out domain.Run
	err := s.InTx(ctx, "claim import run", func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73819,11)`); err != nil {
			return err
		}
		if err := expireExhausted(ctx, tx); err != nil {
			return err
		}
		if err := retireStaleVersions(ctx, tx, p.PipelineVersion); err != nil {
			return err
		}
		var err error
		out, err = claimRun(ctx, tx, p)
		if errors.Is(err, domain.ErrNoWork) {
			return nil
		}
		return err
	})
	if err == nil && out.ID == "" {
		return out, domain.ErrNoWork
	}
	return out, err
}
func claimRun(ctx context.Context, tx pgx.Tx, p domain.ClaimPolicy) (domain.Run, error) {
	var running int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.word_import_runs WHERE status='running' AND lease_until>clock_timestamp()`).Scan(&running); err != nil {
		return domain.Run{}, err
	}
	if running >= p.GlobalLimit {
		return domain.Run{}, domain.ErrNoWork
	}
	var importID, runID string
	err := tx.QueryRow(ctx, `SELECT i.id::text,r.id::text FROM app.word_imports i JOIN app.word_import_runs r ON r.import_id=i.id
 WHERE i.status IN ('queued','processing') AND i.source_revision=r.source_revision AND r.pipeline_version=$1
 AND r.attempt_count<r.max_attempts AND ((r.status='queued' AND r.available_at<=clock_timestamp()) OR (r.status='running' AND r.lease_until<=clock_timestamp()))
 AND (SELECT count(*) FROM app.word_import_runs busy WHERE busy.requested_by=r.requested_by AND busy.status='running' AND busy.lease_until>clock_timestamp())<$2
 ORDER BY r.available_at,r.created_at,r.id FOR UPDATE OF i SKIP LOCKED LIMIT 1`, p.PipelineVersion, p.ActorLimit).Scan(&importID, &runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Run{}, domain.ErrNoWork
	}
	if err != nil {
		return domain.Run{}, err
	}
	previous, err := eligibleRun(ctx, tx, importID, runID, p)
	if err != nil {
		return domain.Run{}, err
	}
	if previous.Status == "running" {
		if err := recordRunEvent(ctx, tx, previous, "lease_expired", "WORKER_LEASE_EXPIRED", nil); err != nil {
			return domain.Run{}, err
		}
	}
	out, err := scanRun(tx.QueryRow(ctx, `UPDATE app.word_import_runs SET status='running',stage='source_validation',claim_token=claim_token+1,attempt_count=attempt_count+1,
 worker_id=$3,lease_until=clock_timestamp()+$4*interval '1 millisecond',error_code=NULL WHERE import_id=$1 AND id=$2 RETURNING `+runColumns, importID, runID, p.WorkerID, p.Lease.Milliseconds()))
	if err != nil {
		return out, err
	}
	if _, err := tx.Exec(ctx, `UPDATE app.word_imports SET status='processing',revision=revision+1 WHERE id=$1`, importID); err != nil {
		return out, err
	}
	return out, recordRunEvent(ctx, tx, out, "claimed", "", nil)
}

type expiredRun struct{ importID, runID string }

const retirementGrace = "10 minutes"

func retireStaleVersions(ctx context.Context, tx pgx.Tx, version string) error {
	rows, err := db.QueryMany(ctx, tx, `SELECT i.id::text,r.id::text FROM app.word_imports i JOIN app.word_import_runs r ON r.import_id=i.id
 WHERE i.status IN ('queued','processing') AND r.pipeline_version<>$1
 AND ((r.status='queued' AND r.available_at<=clock_timestamp()-$2::interval) OR (r.status='running' AND r.lease_until<=clock_timestamp()-$2::interval))
 ORDER BY r.available_at,r.id FOR UPDATE OF i SKIP LOCKED LIMIT 50`, []any{version, retirementGrace}, func(row pgx.Rows) (expiredRun, error) {
		var r expiredRun
		err := row.Scan(&r.importID, &r.runID)
		return r, err
	})
	if err != nil {
		return err
	}
	for _, r := range rows {
		run, err := scanRun(tx.QueryRow(ctx, `UPDATE app.word_import_runs SET status='failed',claim_token=claim_token+1,worker_id=NULL,lease_until=NULL,error_code='PIPELINE_RETIRED',completed_at=clock_timestamp() WHERE id=$1 RETURNING `+runColumns, r.runID))
		if err != nil {
			return err
		}
		if err := recordRunEvent(ctx, tx, run, "failed", "PIPELINE_RETIRED", nil); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.word_imports SET status='failed',revision=revision+1 WHERE id=$1`, r.importID); err != nil {
			return err
		}
	}
	return nil
}

func expireExhausted(ctx context.Context, tx pgx.Tx) error {
	rows, err := db.QueryMany(ctx, tx, `SELECT i.id::text,r.id::text FROM app.word_imports i JOIN app.word_import_runs r ON r.import_id=i.id
 WHERE i.status='processing' AND r.status='running' AND r.lease_until<=clock_timestamp() AND r.attempt_count>=r.max_attempts
 ORDER BY r.lease_until,r.id FOR UPDATE OF i SKIP LOCKED LIMIT 50`, nil, func(row pgx.Rows) (expiredRun, error) {
		var r expiredRun
		err := row.Scan(&r.importID, &r.runID)
		return r, err
	})
	if err != nil {
		return err
	}
	for _, r := range rows {
		run, err := scanRun(tx.QueryRow(ctx, `SELECT `+runColumns+` FROM app.word_import_runs WHERE id=$1 FOR UPDATE`, r.runID))
		if err != nil {
			return err
		}
		if err := recordRunEvent(ctx, tx, run, "lease_expired", "WORKER_LEASE_EXPIRED", nil); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.word_import_runs SET status='failed',claim_token=claim_token+1,worker_id=NULL,lease_until=NULL,error_code='WORKER_LEASE_EXPIRED',completed_at=clock_timestamp() WHERE id=$1`, r.runID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.word_imports SET status='failed',revision=revision+1 WHERE id=$1`, r.importID); err != nil {
			return err
		}
	}
	return nil
}

func eligibleRun(ctx context.Context, tx pgx.Tx, importID, runID string, p domain.ClaimPolicy) (domain.Run, error) {
	run, err := scanRun(tx.QueryRow(ctx, `SELECT `+runColumns+` FROM app.word_import_runs WHERE import_id=$1 AND id=$2 AND pipeline_version=$3 AND attempt_count<max_attempts
 AND ((status='queued' AND available_at<=clock_timestamp()) OR (status='running' AND lease_until<=clock_timestamp())) FOR UPDATE`, importID, runID, p.PipelineVersion))
	if errors.Is(err, domain.ErrNotFound) {
		return run, domain.ErrNoWork
	}
	return run, err
}

func lockedClaim(ctx context.Context, tx pgx.Tx, claim domain.Claim) (domain.Run, error) {
	parent, err := scanImport(tx.QueryRow(ctx, `SELECT `+importColumns+` FROM app.word_imports WHERE id=$1 FOR UPDATE`, claim.ImportID))
	if err != nil {
		return domain.Run{}, err
	}
	if parent.Status != "processing" || parent.SourceRevision != claim.SourceRevision {
		return domain.Run{}, domain.ErrLeaseLost
	}
	run, err := scanRun(tx.QueryRow(ctx, `SELECT `+runColumns+` FROM app.word_import_runs WHERE id=$1 AND import_id=$2 AND source_revision=$3 AND status='running' AND claim_token=$4 AND worker_id=$5 AND lease_until>clock_timestamp() FOR UPDATE`, claim.RunID, claim.ImportID, claim.SourceRevision, claim.Token, claim.WorkerID))
	if errors.Is(err, domain.ErrNotFound) {
		return run, domain.ErrLeaseLost
	}
	return run, err
}
