package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/db"
	"strings"
)

const runColumns = `id::text,import_id::text,source_revision,request_id::text,requested_by::text,expected_revision,pipeline_version,status,stage,attempt_count,max_attempts,claim_token,worker_id::text,lease_until,available_at,error_code,result,created_at,updated_at,completed_at,profile`

func prefixed(alias, columns string) string {
	parts := strings.Split(columns, ",")
	for i, part := range parts {
		parts[i] = alias + "." + part
	}
	return strings.Join(parts, ",")
}

func scanRun(row pgx.Row) (domain.Run, error) {
	var r domain.Run
	var profile []byte
	err := row.Scan(&r.ID, &r.ImportID, &r.SourceRevision, &r.RequestID, &r.RequestedBy, &r.ExpectedRevision, &r.PipelineVersion, &r.Status, &r.Stage, &r.AttemptCount, &r.MaxAttempts, &r.ClaimToken, &r.WorkerID, &r.LeaseUntil, &r.AvailableAt, &r.ErrorCode, &r.Result, &r.CreatedAt, &r.UpdatedAt, &r.CompletedAt, &profile)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, domain.ErrNotFound
	}
	if err != nil {
		return r, err
	}
	return r, json.Unmarshal(profile, &r.Profile)
}
func (s *Postgres) Run(ctx context.Context, importID, id string) (domain.Run, error) {
	return scanRun(s.QueryRow(ctx, `SELECT `+runColumns+` FROM app.word_import_runs WHERE import_id=$1 AND id=$2`, importID, id))
}
func (s *Postgres) Runs(ctx context.Context, importID string) ([]domain.Run, error) {
	if _, err := s.Get(ctx, importID); err != nil {
		return nil, err
	}
	return db.QueryMany(ctx, s, `SELECT `+runColumns+` FROM app.word_import_runs WHERE import_id=$1 ORDER BY created_at DESC,id DESC LIMIT 50`, []any{importID}, func(r pgx.Rows) (domain.Run, error) { return scanRun(r) })
}
func (s *Postgres) Schedule(ctx context.Context, in domain.Schedule) (domain.Run, error) {
	var out domain.Run
	err := s.InTx(ctx, "schedule import", func(tx pgx.Tx) error {
		var err error
		out, err = scheduleRun(ctx, tx, in)
		return err
	})
	return out, err
}
func scheduleRun(ctx context.Context, tx pgx.Tx, in domain.Schedule) (domain.Run, error) {
	parent, err := scanImport(tx.QueryRow(ctx, `SELECT `+importColumns+` FROM app.word_imports WHERE id=$1 FOR UPDATE`, in.ImportID))
	if err != nil {
		return domain.Run{}, err
	}
	previous, err := scanRun(tx.QueryRow(ctx, `SELECT `+runColumns+` FROM app.word_import_runs WHERE import_id=$1 AND request_id=$2`, in.ImportID, in.RequestID))
	if err == nil {
		if previous.SourceRevision != in.SourceRevision || previous.ExpectedRevision != in.ExpectedRevision || previous.PipelineVersion != in.PipelineVersion || previous.MaxAttempts != in.MaxAttempts || previous.Profile != in.Profile {
			return domain.Run{}, domain.ErrConflict
		}
		return previous, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.Run{}, err
	}
	if parent.Revision != in.ExpectedRevision || parent.SourceRevision != in.SourceRevision || !domain.AcceptsSources(parent.Status) {
		return domain.Run{}, domain.ErrConflict
	}
	if err := checkRunnable(ctx, tx, parent); err != nil {
		return domain.Run{}, err
	}
	profile, err := json.Marshal(in.Profile)
	if err != nil {
		return domain.Run{}, err
	}
	out, err := scanRun(tx.QueryRow(ctx, `INSERT INTO app.word_import_runs(import_id,source_revision,request_id,requested_by,expected_revision,pipeline_version,max_attempts,profile) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+runColumns, in.ImportID, in.SourceRevision, in.RequestID, in.Actor.ID, in.ExpectedRevision, in.PipelineVersion, in.MaxAttempts, profile))
	if err != nil {
		return out, err
	}
	if _, err := tx.Exec(ctx, `UPDATE app.word_imports SET status='queued',revision=revision+1 WHERE id=$1`, in.ImportID); err != nil {
		return out, err
	}
	if err := recordRunEvent(ctx, tx, out, "queued", "", &in.Actor.ID); err != nil {
		return out, err
	}
	return out, auditImport(ctx, tx, in.Actor, in.ImportID, "import.queued")
}
func checkRunnable(ctx context.Context, tx pgx.Tx, parent domain.Import) error {
	var exam bool
	var count int
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.word_import_source_set_items WHERE import_id=$1 AND revision=$2 AND role='exam'),(SELECT count(*) FROM app.word_import_runs WHERE import_id=$1)`, parent.ID, parent.SourceRevision).Scan(&exam, &count); err != nil {
		return err
	}
	if !exam {
		return domain.ErrConflict
	}
	if count >= 50 {
		return domain.ErrQuota
	}
	return nil
}
func (s *Postgres) Cancel(ctx context.Context, in domain.Cancel) (domain.Import, error) {
	var out domain.Import
	err := s.InTx(ctx, "cancel import", func(tx pgx.Tx) error {
		parent, err := scanImport(tx.QueryRow(ctx, `SELECT `+importColumns+` FROM app.word_imports WHERE id=$1 FOR UPDATE`, in.ImportID))
		if err != nil {
			return err
		}
		if parent.Status == "cancelled" {
			out, err = hydrateImport(ctx, tx, parent)
			return err
		}
		if parent.Revision != in.ExpectedRevision || parent.Status == "committing" || parent.Status == "committed" {
			return domain.ErrConflict
		}
		stopped, err := cancelActiveRuns(ctx, tx, in)
		if err != nil {
			return err
		}
		var status string
		if err := tx.QueryRow(ctx, `UPDATE app.word_imports SET revision=revision+1,
 status=CASE WHEN $2 AND EXISTS (SELECT 1 FROM app.word_import_drafts WHERE import_id=$1) THEN 'needs_review' ELSE 'cancelled' END
 WHERE id=$1 RETURNING status`, in.ImportID, stopped).Scan(&status); err != nil {
			return err
		}
		action := "import.cancelled"
		if status == "needs_review" {
			action = "import.reprocess_cancelled"
		}
		if err := auditImport(ctx, tx, in.Actor, in.ImportID, action); err != nil {
			return err
		}
		out, err = readImport(ctx, tx, in.ImportID)
		return err
	})
	return out, err
}

func cancelActiveRuns(ctx context.Context, tx pgx.Tx, in domain.Cancel) (bool, error) {
	cancelled, err := db.QueryMany(ctx, tx, `UPDATE app.word_import_runs SET status='cancelled',claim_token=claim_token+1,worker_id=NULL,lease_until=NULL,completed_at=clock_timestamp() WHERE import_id=$1 AND status IN ('queued','running') RETURNING `+runColumns, []any{in.ImportID}, func(row pgx.Rows) (domain.Run, error) { return scanRun(row) })
	if err != nil {
		return false, err
	}
	for _, run := range cancelled {
		if err := recordRunEvent(ctx, tx, run, "cancelled", "", &in.Actor.ID); err != nil {
			return false, err
		}
	}
	return len(cancelled) > 0, nil
}

const failImport = `UPDATE app.word_imports SET revision=revision+1,
 status=CASE WHEN EXISTS (SELECT 1 FROM app.word_import_drafts WHERE import_id=$1) THEN 'needs_review' ELSE 'failed' END WHERE id=$1`
