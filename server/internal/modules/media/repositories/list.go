package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/paging"
	"strings"
)

const (
	// DefaultLimit is a grid's page: 24 fills 3-4 rows at every A-07 width.
	DefaultLimit = 24
	MaxLimit     = 100
)

func library(scope access.Scope, kind *domain.Kind) (string, []any) {
	where, args := []string{"deleted_at IS NULL"}, []any{}
	if !scope.All {
		args = append(args, opt.String(scope.UserID))
		where = append(where, fmt.Sprintf("owner_id = $%d::uuid", len(args)))
	}
	if kind != nil {
		args = append(args, string(*kind))
		where = append(where, fmt.Sprintf("kind = $%d::app.media_kind", len(args)))
	}
	return `
		  FROM app.media_assets
		 WHERE ` + strings.Join(where, "\n		   AND "), args
}

// TotalBytes sums the live assets List pages through for the same scope and
// kind, so the library's header agrees with its rows.
func (s *Postgres) TotalBytes(ctx context.Context, scope access.Scope, kind *domain.Kind) (int64, error) {
	from, args := library(scope, kind)
	var total int64
	if err := s.QueryRow(ctx, `SELECT coalesce(sum(bytes), 0)`+from, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("media: total bytes: %w", err)
	}
	return total, nil
}

// List returns one page of the live assets in the input's scope, newest first,
// with the paging beside it.
func (s *Postgres) List(ctx context.Context, in domain.ListInput) ([]domain.Asset, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	from, args := library(in.Scope, in.Kind)

	page := paging.Page{Number: number, Size: limit}
	if err := s.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("media: count assets: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := s.Query(ctx, `
		SELECT id::text, kind::text, storage_key, mime_type, bytes, duration_ms,
		       original_filename, checksum_sha256, created_at`+from+fmt.Sprintf(`
		 ORDER BY created_at DESC, id DESC
		 LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("media: list assets: %w", err)
	}
	defer rows.Close()

	assets := make([]domain.Asset, 0, limit)
	for rows.Next() {
		var a domain.Asset
		var kind string
		if err := rows.Scan(&a.ID, &kind, &a.StorageKey, &a.MimeType, &a.Bytes,
			&a.DurationMs, &a.OriginalFilename, &a.ChecksumSHA256, &a.CreatedAt); err != nil {
			return nil, paging.Page{}, fmt.Errorf("media: scan asset: %w", err)
		}
		a.Kind = domain.Kind(kind)
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		return nil, paging.Page{}, fmt.Errorf("media: list assets: %w", err)
	}
	return assets, page, nil
}
