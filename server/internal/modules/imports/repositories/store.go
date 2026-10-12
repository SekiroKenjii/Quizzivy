// Package repositories persists import metadata and immutable source revisions in PostgreSQL.
package repositories

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"time"
)

type Postgres struct{ db.Repository }

func NewPostgres(ctx db.Context) *Postgres { return &Postgres{db.NewRepository(ctx)} }

var anyImport = access.Scope{All: true}

const scopedImport = `($2::boolean OR created_by = $3::uuid)`

const importColumns = `id::text, title, status, revision, coalesce(source_revision,0), created_by::text, created_at, updated_at, files_removed_at, closed_idle`
const sourceColumns = `id::text, import_id::text, upload_id::text, expected_revision, role, filename, format, characters, bytes, checksum_sha256, storage_key, uploaded_by::text, ready, coalesce(source_revision,0), created_at`

func scanImport(row pgx.Row) (domain.Import, error) {
	var v domain.Import
	err := row.Scan(&v.ID, &v.Title, &v.Status, &v.Revision, &v.SourceRevision, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt, &v.FilesRemovedAt, &v.ClosedIdle)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, domain.ErrNotFound
	}
	return v, err
}
func scanSource(row pgx.Row) (domain.Source, error) {
	var v domain.Source
	err := row.Scan(&v.ID, &v.ImportID, &v.UploadID, &v.ExpectedRevision, &v.Role, &v.Filename, &v.Format, &v.Characters, &v.Bytes, &v.SHA256, &v.StorageKey, &v.UploadedBy, &v.Ready, &v.SourceRevision, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, domain.ErrNotFound
	}
	return v, err
}

func auditImport(ctx context.Context, tx pgx.Tx, a actor.Actor, id, action string) error {
	return audit.Write(ctx, tx, audit.Entry{ActorUserID: &a.ID, Entity: "word_import", EntityID: &id, Action: action, OccurredAt: time.Now(), IP: a.IPValue(), UserAgent: a.UserAgentValue()})
}

func quotaLock(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73819, 10)`)
	return err
}

func (s *Postgres) Create(ctx context.Context, in domain.Create, quotas domain.Quotas) (domain.Import, error) {
	var out domain.Import
	err := s.InTx(ctx, "create import", func(tx pgx.Tx) error {
		var err error
		out, err = createImport(ctx, tx, in, quotas)
		return err
	})
	if err != nil {
		return out, err
	}
	return s.Get(ctx, anyImport, out.ID)
}
func createImport(ctx context.Context, tx pgx.Tx, in domain.Create, quotas domain.Quotas) (domain.Import, error) {
	if err := quotaLock(ctx, tx); err != nil {
		return domain.Import{}, err
	}
	previous, err := scanImport(tx.QueryRow(ctx, `SELECT `+importColumns+` FROM app.word_imports WHERE created_by=$1 AND request_id=$2`, in.Actor.ID, in.RequestID))
	if err == nil {
		if previous.Title != in.Title {
			return domain.Import{}, domain.ErrConflict
		}
		return previous, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.Import{}, err
	}
	var actorCount, total int
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE created_by=$1), count(*) FROM app.word_imports WHERE status NOT IN ('committed','cancelled')`, in.Actor.ID).Scan(&actorCount, &total); err != nil {
		return domain.Import{}, err
	}
	if actorCount >= quotas.ActorImports || total >= quotas.GlobalImports {
		return domain.Import{}, domain.ErrQuota
	}
	out, err := scanImport(tx.QueryRow(ctx, `INSERT INTO app.word_imports (created_by,request_id,title) VALUES ($1,$2,$3) RETURNING `+importColumns, in.Actor.ID, in.RequestID, in.Title))
	if err != nil {
		return domain.Import{}, err
	}
	return out, auditImport(ctx, tx, in.Actor, out.ID, "import.created")
}

func readImport(ctx context.Context, q db.Querier, scope access.Scope, id string) (domain.Import, error) {
	v, err := scanImport(q.QueryRow(ctx, `SELECT `+importColumns+` FROM app.word_imports WHERE id=$1 AND `+scopedImport, id, scope.All, opt.String(scope.UserID)))
	if err != nil {
		return v, err
	}
	return hydrateImport(ctx, q, v)
}
func hydrateImport(ctx context.Context, q db.Querier, v domain.Import) (domain.Import, error) {
	sources, err := db.QueryMany(ctx, q, `SELECT `+sourceColumns+` FROM app.word_import_sources WHERE id IN (SELECT source_id FROM app.word_import_source_set_items WHERE import_id=$1 AND revision=$2) ORDER BY role`, []any{v.ID, v.SourceRevision}, func(r pgx.Rows) (domain.Source, error) { return scanSource(r) })
	if err != nil {
		return v, err
	}
	v.Sources = sources
	if err := q.QueryRow(ctx, `SELECT count(*) FROM app.word_import_sources WHERE import_id=$1 AND NOT ready`, v.ID).Scan(&v.PendingUploads); err != nil {
		return v, err
	}
	items := []domain.Import{v}
	err = attachProgress(ctx, q, items)
	return items[0], err
}

// Get reads one import the scope's user created, or any under scope.all;
// another creator's answers ErrNotFound exactly as a missing one does.
func (s *Postgres) Get(ctx context.Context, scope access.Scope, id string) (domain.Import, error) {
	var out domain.Import
	err := s.InTx(ctx, "read import", func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY`); err != nil {
			return err
		}
		var err error
		out, err = readImport(ctx, tx, scope, id)
		return err
	})
	return out, err
}

// Source reads one completed source of an import the scope reaches, as Get
// does; a source of another creator's import answers ErrNotFound.
func (s *Postgres) Source(ctx context.Context, scope access.Scope, importID, id string) (domain.Source, error) {
	out, err := scanSource(s.QueryRow(ctx, `SELECT `+sourceColumns+` FROM app.word_import_sources WHERE import_id=$1 AND id=$2 AND ready
 AND import_id IN (SELECT i.id FROM app.word_imports i WHERE i.id=$1 AND ($3::boolean OR i.created_by=$4::uuid))`, importID, id, scope.All, opt.String(scope.UserID)))
	if err != nil {
		return out, fmt.Errorf("read import source: %w", err)
	}
	return out, nil
}

func lockImport(ctx context.Context, tx pgx.Tx, id string, by actor.Actor) (domain.Import, error) {
	return scanImport(tx.QueryRow(ctx, `SELECT `+importColumns+` FROM app.word_imports WHERE id=$1 AND `+scopedImport+` FOR UPDATE`, id, by.Scope.All, opt.String(by.ID)))
}
