package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"

	"github.com/jackc/pgx/v5"
)

type Postgres struct{ db.Repository }

func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

const auditedEntity = "media_asset"

const assetColumns = `a.id::text, a.kind::text, a.storage_key, a.mime_type, a.bytes, a.duration_ms,
		       a.original_filename, a.checksum_sha256, a.created_at,
		       coalesce(a.display_name, a.original_filename), a.default_max_plays::integer, a.width, a.height`

func scanAsset(row pgx.Row) (domain.Asset, error) {
	var a domain.Asset
	var kind string
	err := row.Scan(&a.ID, &kind, &a.StorageKey, &a.MimeType, &a.Bytes, &a.DurationMs,
		&a.OriginalFilename, &a.ChecksumSHA256, &a.CreatedAt,
		&a.DisplayName, &a.DefaultMaxPlays, &a.Width, &a.Height)
	a.Kind = domain.Kind(kind)
	return a, err
}

func ownerOf(in domain.InsertInput) string {
	if in.OwnerID != "" {
		return in.OwnerID
	}
	return in.UploaderID
}

// QuotaCheck is one owner's library measured against Quota with Add more
// bytes in it. LeaveOut, when set, is an asset whose bytes are not counted,
// as the one a new file replaces.
type QuotaCheck struct {
	OwnerID  string
	Add      int64
	Quota    int64
	LeaveOut string
}

// RequireQuota answers ErrQuotaExceeded when the owner's library, with Add
// more bytes, would hold more than Quota. It first takes the owner's
// transaction advisory lock, so two writers of one library decide one after
// the other, and it must run in the transaction that then writes the row.
func RequireQuota(ctx context.Context, tx pgx.Tx, in QuotaCheck) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73820, hashtext($1::uuid::text))`, in.OwnerID); err != nil {
		return fmt.Errorf("media: quota lock: %w", err)
	}
	from, args := library(libraryFilter{scope: access.Scope{UserID: in.OwnerID}})
	args = append(args, opt.String(in.LeaveOut))
	var held int64
	err := tx.QueryRow(ctx, `SELECT coalesce(sum(a.bytes), 0)::bigint`+from+fmt.Sprintf(`
		   AND ($%[1]d::uuid IS NULL OR a.id <> $%[1]d::uuid)`, len(args)), args...).Scan(&held)
	if err != nil {
		return fmt.Errorf("media: quota sum: %w", err)
	}
	return domain.Assets.CheckQuota(held, in.Add, in.Quota)
}

// Insert writes one asset row and its audit row in the caller's transaction.
// The row belongs to in.OwnerID, or to the uploader when that is empty; the
// audit row names the uploader. It checks no quota: RequireQuota does.
func Insert(ctx context.Context, tx pgx.Tx, in domain.InsertInput) (domain.Asset, error) {
	const q = `
		INSERT INTO app.media_assets AS a
		       (id, kind, storage_key, mime_type, bytes, duration_ms,
		        original_filename, checksum_sha256, uploaded_by, created_at, owner_id,
		        display_name, default_max_plays, width, height)
		VALUES ($1, $2::app.media_kind, $3, $4, $5, $6, $7, $8, $9, $10, $11,
		        $12, $13::smallint, $14, $15)
		RETURNING ` + assetColumns

	a, err := scanAsset(tx.QueryRow(ctx, q,
		in.ID, string(in.Kind), in.StorageKey, in.MimeType, in.Bytes, in.DurationMs,
		in.OriginalFilename, in.ChecksumSHA256, in.UploaderID, in.Now, ownerOf(in),
		in.DisplayName, in.DefaultMaxPlays, in.Width, in.Height))
	if err != nil {
		return domain.Asset{}, fmt.Errorf("media: insert asset: %w", err)
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.UploaderID,
		Action:      "media.uploaded",
		Entity:      auditedEntity,
		EntityID:    &a.ID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
	}); err != nil {
		return domain.Asset{}, err
	}
	return a, nil
}

// Insert stores one asset in its owner's library, refusing with
// ErrQuotaExceeded a row that would take the library past in.QuotaBytes. The
// check and the write share one transaction under the owner's quota lock.
func (s *Postgres) Insert(ctx context.Context, in domain.InsertInput) (domain.Asset, error) {
	var a domain.Asset
	err := s.InTx(ctx, "media: insert", func(tx pgx.Tx) error {
		if err := RequireQuota(ctx, tx, QuotaCheck{OwnerID: ownerOf(in), Add: in.Bytes, Quota: in.QuotaBytes}); err != nil {
			return err
		}
		var err error
		a, err = Insert(ctx, tx, in)
		return err
	})
	if err != nil {
		return domain.Asset{}, err
	}
	return a, nil
}

// Get returns one live asset.
func (s *Postgres) Get(ctx context.Context, id string) (domain.Asset, error) {
	a, err := scanAsset(s.QueryRow(ctx, `SELECT `+assetColumns+`
		  FROM app.media_assets a
		 WHERE a.id = $1 AND a.deleted_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Asset{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Asset{}, fmt.Errorf("media: load asset: %w", err)
	}
	return a, nil
}

const libraryAsset = `a.id = $1 AND ` + inLibrary + ` AND ($2::boolean OR a.owner_id = $3::uuid)`

// Find returns one asset of the library in the scope: the scope's own, or
// any under scope.all. A missing, deleted or replaced asset and another
// owner's all answer ErrNotFound.
func (s *Postgres) Find(ctx context.Context, scope access.Scope, id string) (domain.Asset, error) {
	a, err := scanAsset(s.QueryRow(ctx, `SELECT `+assetColumns+`
		  FROM app.media_assets a
		 WHERE `+libraryAsset, id, scope.All, opt.String(scope.UserID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Asset{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Asset{}, fmt.Errorf("media: find asset: %w", err)
	}
	return a, nil
}

// Update writes the display name and the default play limit the input names
// on one asset of the library in the actor's scope and audits it in the same
// transaction. An asset outside the library or the scope answers ErrNotFound
// and nothing is written.
func (s *Postgres) Update(ctx context.Context, in domain.UpdateInput) (domain.Asset, error) {
	var a domain.Asset
	err := s.InTx(ctx, "media: update", func(tx pgx.Tx) error {
		var err error
		a, err = scanAsset(tx.QueryRow(ctx, `
			UPDATE app.media_assets AS a
			   SET display_name = CASE WHEN $4::boolean THEN $5::text ELSE a.display_name END,
			       default_max_plays = CASE WHEN $6::boolean THEN $7::smallint ELSE a.default_max_plays END
			 WHERE `+libraryAsset+`
			RETURNING `+assetColumns,
			in.ID, in.All, opt.String(in.ActorID),
			in.DisplayName != nil, in.DisplayName, in.SetDefaultMaxPlays, in.DefaultMaxPlays))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("media: update asset: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			ActorUserID: &in.ActorID,
			Action:      "media.updated",
			Entity:      auditedEntity,
			EntityID:    &in.ID,
			OccurredAt:  in.Now,
			IP:          opt.String(in.IP),
			UserAgent:   opt.String(in.UserAgent),
		})
	})
	if err != nil {
		return domain.Asset{}, err
	}
	return a, nil
}

// CountByChecksum powers the "you already uploaded this" warning [D-06], so it
// counts only the owner's live assets: another teacher's upload of the same
// bytes is not the caller's to know about. It never blocks a write: §11.1 says
// a re-upload creates a new row.
func (s *Postgres) CountByChecksum(ctx context.Context, ownerID string, checksum []byte) (int, error) {
	var n int
	err := s.QueryRow(ctx,
		`SELECT count(*) FROM app.media_assets
		  WHERE checksum_sha256 = $1 AND owner_id = $2::uuid AND deleted_at IS NULL`,
		checksum, opt.String(ownerID)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("media: count by checksum: %w", err)
	}
	return n, nil
}
