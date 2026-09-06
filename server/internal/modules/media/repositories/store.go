package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/audit"

	"github.com/jackc/pgx/v5"
)

type Postgres struct{ db.Repository }

func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

func (s *Postgres) Insert(ctx context.Context, in domain.InsertInput) (domain.Asset, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("media: begin insert: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const q = `
		INSERT INTO app.media_assets
		       (id, kind, storage_key, mime_type, bytes, duration_ms,
		        original_filename, checksum_sha256, uploaded_by, created_at)
		VALUES ($1, $2::app.media_kind, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id::text, created_at`

	var a domain.Asset
	if err := tx.QueryRow(ctx, q,
		in.ID, string(in.Kind), in.StorageKey, in.MimeType, in.Bytes, in.DurationMs,
		in.OriginalFilename, in.ChecksumSHA256, in.UploaderID, in.Now,
	).Scan(&a.ID, &a.CreatedAt); err != nil {
		return domain.Asset{}, fmt.Errorf("media: insert asset: %w", err)
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.UploaderID,
		Action:      "media.uploaded",
		Entity:      "media_asset",
		EntityID:    &a.ID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
	}); err != nil {
		return domain.Asset{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Asset{}, fmt.Errorf("media: commit insert: %w", err)
	}

	a.Kind = in.Kind
	a.StorageKey = in.StorageKey
	a.MimeType = in.MimeType
	a.Bytes = in.Bytes
	a.DurationMs = in.DurationMs
	a.OriginalFilename = in.OriginalFilename
	a.ChecksumSHA256 = in.ChecksumSHA256
	return a, nil
}

// Get returns one live asset.
func (s *Postgres) Get(ctx context.Context, id string) (domain.Asset, error) {
	const q = `
		SELECT id::text, kind::text, storage_key, mime_type, bytes, duration_ms,
		       original_filename, checksum_sha256, created_at
		  FROM app.media_assets
		 WHERE id = $1 AND deleted_at IS NULL`

	var a domain.Asset
	var kind string
	err := s.QueryRow(ctx, q, id).Scan(
		&a.ID, &kind, &a.StorageKey, &a.MimeType, &a.Bytes, &a.DurationMs,
		&a.OriginalFilename, &a.ChecksumSHA256, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Asset{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Asset{}, fmt.Errorf("media: load asset: %w", err)
	}
	a.Kind = domain.Kind(kind)
	return a, nil
}

// CountByChecksum powers the "you already uploaded this" warning [D-06]. It
// never blocks a write: §11.1 says a re-upload creates a new row.
func (s *Postgres) CountByChecksum(ctx context.Context, checksum []byte) (int, error) {
	var n int
	err := s.QueryRow(ctx,
		`SELECT count(*) FROM app.media_assets
		  WHERE checksum_sha256 = $1 AND deleted_at IS NULL`, checksum).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("media: count by checksum: %w", err)
	}
	return n, nil
}
