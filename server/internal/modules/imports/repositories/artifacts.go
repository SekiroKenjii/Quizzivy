package repositories

import (
	"bytes"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/db"
)

const artifactSetColumns = `id::text,import_id::text,run_id::text,source_revision,source_id::text,role,claim_token,stage,component_version,plan_digest,manifest,file_count,bytes,ready,created_at,completed_at`
const artifactColumns = `id::text,set_id::text,import_id::text,name,kind,content_type,bytes,checksum_sha256,storage_key,ready`

func scanArtifactSet(row pgx.Row) (domain.ArtifactSet, error) {
	var v domain.ArtifactSet
	err := row.Scan(&v.ID, &v.ImportID, &v.RunID, &v.SourceRevision, &v.SourceID, &v.Role, &v.ClaimToken, &v.Stage, &v.ComponentVersion, &v.PlanDigest, &v.Manifest, &v.FileCount, &v.Bytes, &v.Ready, &v.CreatedAt, &v.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, domain.ErrNotFound
	}
	return v, err
}
func scanArtifact(row pgx.Rows) (domain.Artifact, error) {
	var v domain.Artifact
	err := row.Scan(&v.ID, &v.SetID, &v.ImportID, &v.Name, &v.Kind, &v.ContentType, &v.Bytes, &v.SHA256, &v.StorageKey, &v.Ready)
	return v, err
}
func hydrateArtifactSet(ctx context.Context, q db.Querier, v domain.ArtifactSet) (domain.ArtifactSet, error) {
	files, err := db.QueryMany(ctx, q, `SELECT `+artifactColumns+` FROM app.word_import_artifacts WHERE set_id=$1 AND import_id=$2 ORDER BY ordinal`, []any{v.ID, v.ImportID}, scanArtifact)
	v.Files = files
	return v, err
}

func (s *Postgres) ReserveArtifacts(ctx context.Context, c domain.Claim, p domain.ArtifactPlan, quotas domain.ArtifactQuotas) (domain.ArtifactSet, error) {
	digest, total, err := p.Digest()
	if err != nil {
		return domain.ArtifactSet{}, err
	}
	if quotas.ActorBytes <= 0 || quotas.GlobalBytes <= 0 || quotas.SetsPerImport <= 0 {
		return domain.ArtifactSet{}, domain.ErrQuota
	}
	var out domain.ArtifactSet
	err = s.InTx(ctx, "reserve import artifacts", func(tx pgx.Tx) error {
		var err error
		out, err = reserveArtifacts(ctx, tx, c, p, quotas, digest, total)
		return err
	})
	return out, err
}

func reserveArtifacts(ctx context.Context, tx pgx.Tx, c domain.Claim, p domain.ArtifactPlan, quotas domain.ArtifactQuotas, digest []byte, total int64) (domain.ArtifactSet, error) {
	if err := quotaLock(ctx, tx); err != nil {
		return domain.ArtifactSet{}, err
	}
	if _, err := lockedClaim(ctx, tx, c); err != nil {
		return domain.ArtifactSet{}, err
	}
	previous, err := scanArtifactSet(tx.QueryRow(ctx, `SELECT `+artifactSetColumns+` FROM app.word_import_artifact_sets WHERE run_id=$1 AND claim_token=$2 AND role=$3 AND stage=$4 AND component_version=$5`, c.RunID, c.Token, p.Role, p.Stage, p.ComponentVersion))
	if err == nil {
		if !bytes.Equal(previous.PlanDigest, digest) {
			return domain.ArtifactSet{}, domain.ErrConflict
		}
		return hydrateArtifactSet(ctx, tx, previous)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.ArtifactSet{}, err
	}
	var owns bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.word_import_source_set_items WHERE import_id=$1 AND revision=$2 AND role=$3 AND source_id=$4)`, c.ImportID, c.SourceRevision, p.Role, p.SourceID).Scan(&owns); err != nil {
		return domain.ArtifactSet{}, err
	}
	if !owns {
		return domain.ArtifactSet{}, domain.ErrNotFound
	}
	if err := artifactQuota(ctx, tx, c.ImportID, total, quotas); err != nil {
		return domain.ArtifactSet{}, err
	}
	return insertArtifacts(ctx, tx, c, p, digest, total)
}

func insertArtifacts(ctx context.Context, tx pgx.Tx, c domain.Claim, p domain.ArtifactPlan, digest []byte, total int64) (domain.ArtifactSet, error) {
	out, err := scanArtifactSet(tx.QueryRow(ctx, `INSERT INTO app.word_import_artifact_sets(import_id,run_id,source_revision,source_id,role,claim_token,stage,component_version,plan_digest,manifest,file_count,bytes)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING `+artifactSetColumns, c.ImportID, c.RunID, c.SourceRevision, p.SourceID, p.Role, c.Token, p.Stage, p.ComponentVersion, digest, p.Manifest, len(p.Files), total))
	if err != nil {
		return domain.ArtifactSet{}, err
	}
	for ordinal, f := range p.Files {
		if _, err := tx.Exec(ctx, `WITH identity AS (SELECT uuidv7() AS id)
 INSERT INTO app.word_import_artifacts(id,set_id,import_id,ordinal,name,kind,content_type,bytes,checksum_sha256,storage_key)
 SELECT id,$1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,'artifacts/'||$2::uuid::text||'/'||$1::uuid::text||'/'||id::text FROM identity`, out.ID, c.ImportID, ordinal, f.Name, f.Kind, f.ContentType, f.Bytes, f.SHA256); err != nil {
			return domain.ArtifactSet{}, err
		}
	}
	return hydrateArtifactSet(ctx, tx, out)
}

func artifactQuota(ctx context.Context, tx pgx.Tx, importID string, total int64, q domain.ArtifactQuotas) error {
	var actorBytes, globalBytes int64
	var sets int
	if err := tx.QueryRow(ctx, `SELECT coalesce(sum(a.bytes) FILTER(WHERE i.created_by=(SELECT created_by FROM app.word_imports WHERE id=$1) AND i.status NOT IN ('committed','cancelled')),0),
 coalesce(sum(a.bytes) FILTER(WHERE i.status NOT IN ('committed','cancelled')),0),count(*) FILTER(WHERE a.import_id=$1)
 FROM app.word_import_artifact_sets a JOIN app.word_imports i ON i.id=a.import_id`, importID).Scan(&actorBytes, &globalBytes, &sets); err != nil {
		return err
	}
	if total > q.ActorBytes-actorBytes || total > q.GlobalBytes-globalBytes || sets >= q.SetsPerImport {
		return domain.ErrQuota
	}
	return nil
}

func ownedArtifactSet(ctx context.Context, tx pgx.Tx, c domain.Claim, id string) (domain.ArtifactSet, error) {
	if _, err := lockedClaim(ctx, tx, c); err != nil {
		return domain.ArtifactSet{}, err
	}
	return scanArtifactSet(tx.QueryRow(ctx, `SELECT `+artifactSetColumns+` FROM app.word_import_artifact_sets WHERE id=$1 AND import_id=$2 AND run_id=$3 AND claim_token=$4`, id, c.ImportID, c.RunID, c.Token))
}

func (s *Postgres) ArtifactStored(ctx context.Context, c domain.Claim, setID, artifactID string) error {
	return s.InTx(ctx, "acknowledge import artifact", func(tx pgx.Tx) error {
		set, err := ownedArtifactSet(ctx, tx, c, setID)
		if err != nil {
			return err
		}
		var ready bool
		if err := tx.QueryRow(ctx, `SELECT ready FROM app.word_import_artifacts WHERE id=$1 AND set_id=$2 AND import_id=$3`, artifactID, setID, c.ImportID).Scan(&ready); errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		} else if err != nil {
			return err
		}
		if ready {
			return nil
		}
		if set.Ready {
			return domain.ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE app.word_import_artifacts SET ready=true WHERE id=$1`, artifactID)
		return err
	})
}

func (s *Postgres) FinishArtifacts(ctx context.Context, c domain.Claim, id string) (domain.ArtifactSet, error) {
	var out domain.ArtifactSet
	err := s.InTx(ctx, "complete import artifacts", func(tx pgx.Tx) error {
		var err error
		out, err = ownedArtifactSet(ctx, tx, c, id)
		if err != nil {
			return err
		}
		out, err = hydrateArtifactSet(ctx, tx, out)
		if err != nil {
			return err
		}
		if out.Ready {
			return nil
		}
		if len(out.Files) != out.FileCount {
			return domain.ErrConflict
		}
		var total int64
		for _, f := range out.Files {
			if !f.Ready {
				return domain.ErrConflict
			}
			total += f.Bytes
		}
		if total != out.Bytes {
			return domain.ErrConflict
		}
		files := out.Files
		out, err = scanArtifactSet(tx.QueryRow(ctx, `UPDATE app.word_import_artifact_sets SET ready=true,completed_at=clock_timestamp() WHERE id=$1 RETURNING `+artifactSetColumns, id))
		out.Files = files
		return err
	})
	return out, err
}

func (s *Postgres) ReusableArtifacts(ctx context.Context, c domain.Claim, role, stage, version string) (domain.ArtifactSet, error) {
	var out domain.ArtifactSet
	err := s.InTx(ctx, "reuse import artifacts", func(tx pgx.Tx) error {
		run, err := lockedClaim(ctx, tx, c)
		if err != nil {
			return err
		}
		out, err = scanArtifactSet(tx.QueryRow(ctx, `SELECT `+artifactSetColumns+` FROM app.word_import_artifact_sets WHERE import_id=$1 AND source_revision=$2 AND role=$3 AND stage=$4 AND component_version=$5 AND ready
 AND run_id IN (SELECT id FROM app.word_import_runs WHERE import_id=$1 AND pipeline_version=$6) ORDER BY created_at DESC,id DESC LIMIT 1`, c.ImportID, c.SourceRevision, role, stage, version, run.PipelineVersion))
		if err != nil {
			return err
		}
		out, err = hydrateArtifactSet(ctx, tx, out)
		return err
	})
	return out, err
}

func (s *Postgres) ArtifactSet(ctx context.Context, importID, id string) (domain.ArtifactSet, error) {
	set, err := scanArtifactSet(s.QueryRow(ctx, `SELECT `+artifactSetColumns+` FROM app.word_import_artifact_sets WHERE id=$1 AND import_id=$2 AND ready`, id, importID))
	if err != nil {
		return set, err
	}
	return hydrateArtifactSet(ctx, s, set)
}
