package repositories

import (
	"bytes"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/imports/domain"
)

const awaitingSources = "awaiting_sources"

func (s *Postgres) Reserve(ctx context.Context, in domain.Reserve, quotas domain.Quotas) (domain.Source, error) {
	var out domain.Source
	err := s.InTx(ctx, "reserve import source", func(tx pgx.Tx) error {
		var err error
		out, err = reserveSource(ctx, tx, in, quotas)
		return err
	})
	return out, err
}
func reserveSource(ctx context.Context, tx pgx.Tx, in domain.Reserve, quotas domain.Quotas) (domain.Source, error) {
	if err := quotaLock(ctx, tx); err != nil {
		return domain.Source{}, err
	}
	parent, err := scanImport(tx.QueryRow(ctx, `SELECT `+importColumns+` FROM app.word_imports WHERE id=$1 FOR UPDATE`, in.Source.ImportID))
	if err != nil {
		return domain.Source{}, err
	}
	previous, err := scanSource(tx.QueryRow(ctx, `SELECT `+sourceColumns+` FROM app.word_import_sources WHERE import_id=$1 AND upload_id=$2`, in.Source.ImportID, in.Source.UploadID))
	if err == nil {
		return previous, reuseSource(parent, previous, in.Source)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.Source{}, err
	}
	if parent.Status != awaitingSources || parent.Revision != in.Source.ExpectedRevision {
		return domain.Source{}, domain.ErrConflict
	}
	if err := sourceQuota(ctx, tx, in, quotas); err != nil {
		return domain.Source{}, err
	}
	v := in.Source
	out, err := scanSource(tx.QueryRow(ctx, `WITH identity AS (SELECT uuidv7() AS id)
 INSERT INTO app.word_import_sources(id,import_id,upload_id,expected_revision,role,filename,format,bytes,checksum_sha256,storage_key,uploaded_by)
 SELECT id,$1::uuid,$2,$3,$4,$5,$6,$7,$8,'originals/' || $1::uuid::text || '/' || id::text,$9 FROM identity RETURNING `+sourceColumns,
		v.ImportID, v.UploadID, v.ExpectedRevision, v.Role, v.Filename, v.Format, v.Bytes, v.SHA256, in.Actor.ID))
	if err != nil {
		return domain.Source{}, err
	}
	return out, auditImport(ctx, tx, in.Actor, parent.ID, "import.source_reserved")
}
func reuseSource(parent domain.Import, previous, in domain.Source) error {
	if !sameUpload(previous, in) {
		return domain.ErrConflict
	}
	if !previous.Ready && (parent.Status != awaitingSources || parent.Revision != previous.ExpectedRevision) {
		return domain.ErrConflict
	}
	return nil
}
func sourceQuota(ctx context.Context, tx pgx.Tx, in domain.Reserve, quotas domain.Quotas) error {
	var actorBytes, totalBytes int64
	var sourceCount int
	if err := tx.QueryRow(ctx, `SELECT coalesce(sum(bytes) FILTER (WHERE uploaded_by=$1),0), coalesce(sum(bytes),0), count(*) FILTER (WHERE import_id=$2) FROM app.word_import_sources`, in.Actor.ID, in.Source.ImportID).Scan(&actorBytes, &totalBytes, &sourceCount); err != nil {
		return err
	}
	if in.Source.Bytes > quotas.ActorBytes-actorBytes || in.Source.Bytes > quotas.GlobalBytes-totalBytes || sourceCount >= quotas.SourcesPerImport {
		return domain.ErrQuota
	}
	return nil
}

func sameUpload(a, b domain.Source) bool {
	return a.Role == b.Role && a.Filename == b.Filename && a.Format == b.Format && a.Bytes == b.Bytes && a.ExpectedRevision == b.ExpectedRevision && bytes.Equal(a.SHA256, b.SHA256)
}

func (s *Postgres) Finish(ctx context.Context, in domain.Finish) (domain.Receipt, error) {
	var out domain.Receipt
	err := s.InTx(ctx, "finish import source", func(tx pgx.Tx) error {
		parent, err := scanImport(tx.QueryRow(ctx, `SELECT `+importColumns+` FROM app.word_imports WHERE id=$1 FOR UPDATE`, in.ImportID))
		if err != nil {
			return err
		}
		src, err := scanSource(tx.QueryRow(ctx, `SELECT `+sourceColumns+` FROM app.word_import_sources WHERE import_id=$1 AND id=$2`, in.ImportID, in.SourceID))
		if err != nil {
			return err
		}
		if !src.Ready {
			if parent.Status != awaitingSources || parent.Revision != src.ExpectedRevision {
				return domain.ErrConflict
			}
			if err := finishSource(ctx, tx, parent, src, in); err != nil {
				return err
			}
			src.Ready = true
			src.SourceRevision = parent.SourceRevision + 1
		}
		out.Source = src
		out.Import, err = readImport(ctx, tx, parent.ID)
		return err
	})
	return out, err
}

func finishSource(ctx context.Context, tx pgx.Tx, parent domain.Import, src domain.Source, in domain.Finish) error {
	revision := parent.SourceRevision + 1
	if _, err := tx.Exec(ctx, `INSERT INTO app.word_import_source_sets(import_id,revision,created_by) VALUES($1,$2,$3)`, parent.ID, revision, in.Actor.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE app.word_import_sources SET ready=true,source_revision=$3 WHERE import_id=$1 AND id=$2`, parent.ID, src.ID, revision); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.word_import_source_set_items(import_id,revision,role,source_id)
 SELECT import_id,$2,role,source_id FROM app.word_import_source_set_items WHERE import_id=$1 AND revision=$3 AND role<>$4`, parent.ID, revision, parent.SourceRevision, src.Role); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.word_import_source_set_items(import_id,revision,role,source_id) VALUES($1,$2,$3,$4)`, parent.ID, revision, src.Role, src.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE app.word_imports SET revision=revision+1,source_revision=$2 WHERE id=$1`, parent.ID, revision); err != nil {
		return err
	}
	return auditImport(ctx, tx, in.Actor, parent.ID, "import.source_completed")
}
