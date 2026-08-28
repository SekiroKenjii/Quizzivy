package media

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/audit"
)

var ErrNotFound = errors.New("media: asset not found")

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// newAssetID mints the id up front, because the storage key contains it -- the
// object has to be written before the row exists, so the row cannot supply it.
func newAssetID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", errNoID
	}
	return id.String(), nil
}

type InsertInput struct {
	ID               string
	Kind             Kind
	StorageKey       string
	MimeType         string
	Bytes            int64
	DurationMs       *int
	OriginalFilename string
	ChecksumSHA256   []byte
	UploaderID       string
	Now              time.Time
	IP               *string
	UserAgent        *string
}

func (s *Store) Insert(ctx context.Context, in InsertInput) (Asset, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Asset{}, fmt.Errorf("media: begin insert: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const q = `
		INSERT INTO app.media_assets
		       (id, kind, storage_key, mime_type, bytes, duration_ms,
		        original_filename, checksum_sha256, uploaded_by, created_at)
		VALUES ($1, $2::app.media_kind, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id::text, created_at`

	var a Asset
	if err := tx.QueryRow(ctx, q,
		in.ID, string(in.Kind), in.StorageKey, in.MimeType, in.Bytes, in.DurationMs,
		in.OriginalFilename, in.ChecksumSHA256, in.UploaderID, in.Now,
	).Scan(&a.ID, &a.CreatedAt); err != nil {
		return Asset{}, fmt.Errorf("media: insert asset: %w", err)
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
		return Asset{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Asset{}, fmt.Errorf("media: commit insert: %w", err)
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
func (s *Store) Get(ctx context.Context, id string) (Asset, error) {
	const q = `
		SELECT id::text, kind::text, storage_key, mime_type, bytes, duration_ms,
		       original_filename, checksum_sha256, created_at
		  FROM app.media_assets
		 WHERE id = $1 AND deleted_at IS NULL`

	var a Asset
	var kind string
	err := s.pool.QueryRow(ctx, q, id).Scan(
		&a.ID, &kind, &a.StorageKey, &a.MimeType, &a.Bytes, &a.DurationMs,
		&a.OriginalFilename, &a.ChecksumSHA256, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Asset{}, ErrNotFound
	}
	if err != nil {
		return Asset{}, fmt.Errorf("media: load asset: %w", err)
	}
	a.Kind = Kind(kind)
	return a, nil
}

// CountByChecksum powers the "you already uploaded this" warning [D-06]. It
// never blocks a write: §11.1 says a re-upload creates a new row.
func (s *Store) CountByChecksum(ctx context.Context, checksum []byte) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM app.media_assets
		  WHERE checksum_sha256 = $1 AND deleted_at IS NULL`, checksum).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("media: count by checksum: %w", err)
	}
	return n, nil
}
