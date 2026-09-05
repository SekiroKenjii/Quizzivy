package media

import (
	"context"
	"fmt"
	"quizzivy/internal/paging"
)

const (
	// DefaultLimit is a grid's page: 24 fills 3-4 rows at every A-07 width.
	DefaultLimit = 24
	MaxLimit     = 100
)

// ListInput selects a page of the library.
type ListInput struct {
	Kind  *Kind
	Page  int // 1-based; below 1 reads as the first
	Limit int // clamped to [1, MaxLimit]
}

// List returns one page of live assets, newest first, with the paging
// beside it.
//
// OFFSET rather than keyset (O-20 overrides §13.8 here): the teacher wants
// numbered pages, and at this library's size an upload landing mid-pagination
// shifting a page by one row is a smaller cost than a grid that cannot jump.
// TotalBytes sums every live asset the kind filter matches, for the library's
// subtitle: the whole shelf, not the page on screen.
func (s *Store) TotalBytes(ctx context.Context, kind *Kind) (int64, error) {
	var kindArg *string
	if kind != nil {
		k := string(*kind)
		kindArg = &k
	}
	var total int64
	if err := s.pool.QueryRow(ctx, `
		SELECT coalesce(sum(bytes), 0) FROM app.media_assets
		 WHERE deleted_at IS NULL
		   AND ($1::app.media_kind IS NULL OR kind = $1::app.media_kind)`, kindArg).Scan(&total); err != nil {
		return 0, fmt.Errorf("media: total bytes: %w", err)
	}
	return total, nil
}

func (s *Store) List(ctx context.Context, in ListInput) ([]Asset, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	var kindArg *string
	if in.Kind != nil {
		k := string(*in.Kind)
		kindArg = &k
	}
	const from = `
		  FROM app.media_assets
		 WHERE deleted_at IS NULL
		   AND ($1::app.media_kind IS NULL OR kind = $1::app.media_kind)`

	page := paging.Page{Number: number, Size: limit}
	if err := s.pool.QueryRow(ctx, `SELECT count(*)`+from, kindArg).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("media: count assets: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id::text, kind::text, storage_key, mime_type, bytes, duration_ms,
		       original_filename, checksum_sha256, created_at`+from+`
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2 OFFSET $3`, kindArg, limit, offset)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("media: list assets: %w", err)
	}
	defer rows.Close()

	assets := make([]Asset, 0, limit)
	for rows.Next() {
		var a Asset
		var kind string
		if err := rows.Scan(&a.ID, &kind, &a.StorageKey, &a.MimeType, &a.Bytes,
			&a.DurationMs, &a.OriginalFilename, &a.ChecksumSHA256, &a.CreatedAt); err != nil {
			return nil, paging.Page{}, fmt.Errorf("media: scan asset: %w", err)
		}
		a.Kind = Kind(kind)
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		return nil, paging.Page{}, fmt.Errorf("media: list assets: %w", err)
	}
	return assets, page, nil
}
