package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/platform/db"
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

const inLibrary = `a.deleted_at IS NULL AND a.replaced_by IS NULL`

const nameSearch = `app.immutable_unaccent(lower(coalesce(a.display_name, '') || ' ' || a.original_filename))` +
	` LIKE '%%' || app.immutable_unaccent(lower($%[1]d)) || '%%' ESCAPE '\'`

type libraryFilter struct {
	scope  access.Scope
	kind   *domain.Kind
	query  string
	unused bool
}

func filterOf(in domain.ListInput) libraryFilter {
	return libraryFilter{scope: in.Scope, kind: in.Kind, query: in.Query, unused: in.Unused}
}

func library(f libraryFilter) (string, []any) {
	where, args := []string{inLibrary}, []any{}
	if !f.scope.All {
		args = append(args, opt.String(f.scope.UserID))
		where = append(where, fmt.Sprintf("a.owner_id = $%d::uuid", len(args)))
	}
	if f.kind != nil {
		args = append(args, string(*f.kind))
		where = append(where, fmt.Sprintf("a.kind = $%d::app.media_kind", len(args)))
	}
	if q := strings.TrimSpace(f.query); q != "" {
		args = append(args, db.EscapeLike(q))
		where = append(where, fmt.Sprintf(nameSearch, len(args)))
	}
	if f.unused {
		where = append(where, "NOT EXISTS "+questionsUsing)
	}
	return `
		  FROM app.media_assets a
		 WHERE ` + strings.Join(where, "\n		   AND "), args
}

// TotalBytes sums the library assets List pages through for the same scope,
// search and filters, so the library's header agrees with its rows.
func (s *Postgres) TotalBytes(ctx context.Context, in domain.ListInput) (int64, error) {
	from, args := library(filterOf(in))
	var total int64
	if err := s.QueryRow(ctx, `SELECT coalesce(sum(a.bytes), 0)::bigint`+from, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("media: total bytes: %w", err)
	}
	return total, nil
}

// Facets counts the library by tab for the input's scope and search, ignoring
// its kind and its unused filter, so choosing a tab leaves the other counts
// standing. Unused counts the assets no live question uses, as the unused
// filter keeps them.
func (s *Postgres) Facets(ctx context.Context, in domain.ListInput) (domain.Facets, error) {
	from, args := library(libraryFilter{scope: in.Scope, query: in.Query})
	var f domain.Facets
	err := s.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE x.kind = 'audio'),
		       count(*) FILTER (WHERE x.kind = 'image'),
		       count(*) FILTER (WHERE x.unused)
		  FROM (SELECT a.kind, NOT EXISTS `+questionsUsing+` AS unused`+from+`) x`, args...).
		Scan(&f.All, &f.Audio, &f.Image, &f.Unused)
	if err != nil {
		return domain.Facets{}, fmt.Errorf("media: facets: %w", err)
	}
	return f, nil
}

// Usage sums the scope's whole library by kind, whatever the list is
// filtered by. It leaves QuotaBytes zero: the quota is the application's.
func (s *Postgres) Usage(ctx context.Context, scope access.Scope) (domain.Usage, error) {
	from, args := library(libraryFilter{scope: scope})
	var u domain.Usage
	err := s.QueryRow(ctx, `
		SELECT coalesce(sum(a.bytes) FILTER (WHERE a.kind = 'audio'), 0)::bigint,
		       coalesce(sum(a.bytes) FILTER (WHERE a.kind = 'image'), 0)::bigint`+from, args...).
		Scan(&u.AudioBytes, &u.ImageBytes)
	if err != nil {
		return domain.Usage{}, fmt.Errorf("media: usage: %w", err)
	}
	return u, nil
}

// List returns one page of the library in the input's scope, newest first,
// with the paging beside it.
func (s *Postgres) List(ctx context.Context, in domain.ListInput) ([]domain.Asset, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	from, args := library(filterOf(in))

	page := paging.Page{Number: number, Size: limit}
	if err := s.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("media: count assets: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := s.Query(ctx, `SELECT `+assetColumns+from+fmt.Sprintf(`
		 ORDER BY a.created_at DESC, a.id DESC
		 LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("media: list assets: %w", err)
	}
	defer rows.Close()

	assets := make([]domain.Asset, 0, limit)
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, paging.Page{}, fmt.Errorf("media: scan asset: %w", err)
		}
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		return nil, paging.Page{}, fmt.Errorf("media: list assets: %w", err)
	}
	return assets, page, nil
}
