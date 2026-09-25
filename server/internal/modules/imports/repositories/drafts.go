package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

const maxDraftBytes = 8 << 20

func (s *Postgres) Draft(ctx context.Context, importID string) (domain.StoredDraft, error) {
	return readDraft(ctx, s, importID)
}

func readDraft(ctx context.Context, q db.Querier, importID string) (domain.StoredDraft, error) {
	var out domain.StoredDraft
	var body []byte
	err := q.QueryRow(ctx, `SELECT d.import_id::text, i.title, i.status, d.revision, d.body, d.candidate IS NOT NULL, d.updated_at
 FROM app.word_import_drafts d JOIN app.word_imports i ON i.id=d.import_id WHERE d.import_id=$1`, importID).Scan(&out.ImportID, &out.Title, &out.Status, &out.Revision, &body, &out.Reprocessed, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, missingDraft(ctx, q, importID)
	}
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(body, &out.Draft)
}

func missingDraft(ctx context.Context, q db.Querier, importID string) error {
	exists, err := db.Exists(ctx, q, `SELECT 1 FROM app.word_imports WHERE id=$1`, importID)
	if err != nil {
		return err
	}
	if !exists {
		return domain.ErrNotFound
	}
	return domain.ErrNoDraft
}

func (s *Postgres) SaveDraft(ctx context.Context, in domain.SaveDraft) (domain.StoredDraft, error) {
	body, err := json.Marshal(in.Draft)
	if err != nil {
		return domain.StoredDraft{}, err
	}
	if len(body) > maxDraftBytes {
		return domain.StoredDraft{}, domain.ErrTooLarge
	}
	var out domain.StoredDraft
	err = s.InTx(ctx, "save import draft", func(tx pgx.Tx) error {
		if err := lockUnderReview(ctx, tx, in.ImportID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE app.word_import_drafts SET body=$2, revision=revision+1, edited_by=$3 WHERE import_id=$1 AND revision=$4`, in.ImportID, body, in.Actor.ID, in.ExpectedRevision)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			if _, err := readDraft(ctx, tx, in.ImportID); err != nil {
				return err
			}
			return domain.ErrStale
		}
		out, err = readDraft(ctx, tx, in.ImportID)
		return err
	})
	return out, err
}

func lockUnderReview(ctx context.Context, tx pgx.Tx, importID string) error {
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM app.word_imports WHERE id=$1 FOR UPDATE`, importID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "needs_review" {
		return domain.ErrConflict
	}
	return nil
}

func storeMachineDraft(ctx context.Context, tx pgx.Tx, c domain.Claim, draft json.RawMessage) error {
	if len(draft) == 0 || len(draft) > maxDraftBytes || !json.Valid(draft) {
		return domain.ErrInvalidResult
	}
	_, err := tx.Exec(ctx, `INSERT INTO app.word_import_drafts AS d (import_id, run_id, body) VALUES ($1,$2,$3)
 ON CONFLICT (import_id) DO UPDATE SET
   body = CASE WHEN d.edited_by IS NULL THEN EXCLUDED.body ELSE d.body END,
   run_id = CASE WHEN d.edited_by IS NULL THEN EXCLUDED.run_id ELSE d.run_id END,
   revision = CASE WHEN d.edited_by IS NULL THEN d.revision + 1 ELSE d.revision END,
   candidate = CASE WHEN d.edited_by IS NULL THEN NULL ELSE EXCLUDED.body END,
   candidate_run_id = CASE WHEN d.edited_by IS NULL THEN NULL ELSE EXCLUDED.run_id END`, c.ImportID, c.RunID, draft)
	return err
}

func (s *Postgres) Commit(ctx context.Context, importID string) (domain.Commit, error) {
	var out domain.Commit
	err := s.QueryRow(ctx, `SELECT import_id::text, request_id::text, draft_revision, digest, test_id::text FROM app.word_import_commits WHERE import_id=$1`, importID).
		Scan(&out.ImportID, &out.RequestID, &out.DraftRevision, &out.Digest, &out.TestID)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, domain.ErrNotFound
	}
	return out, err
}

func (s *Postgres) RecordCommit(ctx context.Context, in domain.CommitRecord) error {
	return s.InTx(ctx, "record import commit", func(tx pgx.Tx) error {
		if err := lockUnderReview(ctx, tx, in.ImportID); err != nil {
			return err
		}
		var revision int64
		if err := tx.QueryRow(ctx, `SELECT revision FROM app.word_import_drafts WHERE import_id=$1`, in.ImportID).Scan(&revision); err != nil {
			return err
		}
		if revision != in.DraftRevision {
			return domain.ErrStale
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.word_import_commits (import_id, request_id, draft_revision, digest, test_id, committed_by) VALUES ($1,$2,$3,$4,$5,$6)`,
			in.ImportID, in.RequestID, in.DraftRevision, in.Digest, in.TestID, in.Actor.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.word_imports SET status='committed', revision=revision+1 WHERE id=$1`, in.ImportID); err != nil {
			return err
		}
		return auditImport(ctx, tx, in.Actor, in.ImportID, "import.committed")
	})
}

func (s *Postgres) DraftRun(ctx context.Context, importID string) (domain.Run, error) {
	return scanRun(s.QueryRow(ctx, `SELECT `+prefixed("r", runColumns)+` FROM app.word_import_drafts d JOIN app.word_import_runs r ON r.id=d.run_id AND r.import_id=d.import_id WHERE d.import_id=$1`, importID))
}
